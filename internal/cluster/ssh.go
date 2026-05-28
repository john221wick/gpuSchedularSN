package cluster

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

type SSHConfig struct {
	User    string
	Host    string
	Port    int
	KeyPath string
}

// ParseSSHCommand parses raw SSH commands from cloud providers.
// Examples:
//
//	"ssh -p 20544 root@203.0.113.10"
//	"ssh -p 41922 root@ssh5.vast.ai -L 8080:localhost:8080"
//	"ssh root@192.168.1.100"
func ParseSSHCommand(raw string) (*SSHConfig, error) {
	tokens := strings.Fields(strings.TrimSpace(raw))
	if len(tokens) == 0 {
		return nil, fmt.Errorf("empty ssh command")
	}

	// Skip leading "ssh" if present
	start := 0
	if tokens[0] == "ssh" {
		start = 1
	}

	config := &SSHConfig{Port: 22}
	var userHost string

	for i := start; i < len(tokens); i++ {
		tok := tokens[i]
		switch {
		case tok == "-p" && i+1 < len(tokens):
			i++
			p, err := strconv.Atoi(tokens[i])
			if err != nil {
				return nil, fmt.Errorf("invalid port: %s", tokens[i])
			}
			config.Port = p
		case tok == "-i" && i+1 < len(tokens):
			i++
			config.KeyPath = tokens[i]
		case tok == "-L" || tok == "-R" || tok == "-D" || tok == "-o" || tok == "-J":
			// Skip flag + its argument
			i++
		case strings.HasPrefix(tok, "-"):
			// Skip unknown flags
		default:
			// This should be user@host
			if userHost == "" {
				userHost = tok
			}
		}
	}

	if userHost == "" {
		return nil, fmt.Errorf("no user@host found in: %s", raw)
	}

	parts := strings.SplitN(userHost, "@", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("expected user@host, got: %s", userHost)
	}

	config.User = parts[0]
	config.Host = parts[1]

	// Handle host:port format
	if host, port, err := net.SplitHostPort(config.Host); err == nil {
		config.Host = host
		if p, err := strconv.Atoi(port); err == nil {
			config.Port = p
		}
	}

	return config, nil
}

type SSHSession struct {
	Client      *ssh.Client
	Config      SSHConfig
	LocalPort   int
	forwardDone chan struct{}
	mu          sync.Mutex
}

// Connect establishes an SSH connection.
func Connect(config SSHConfig) (*SSHSession, error) {
	key, err := loadPrivateKey(config.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("load key: %w", err)
	}

	sshConfig := &ssh.ClientConfig{
		User: config.User,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(key),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("ssh dial %s: %w", addr, err)
	}

	return &SSHSession{
		Client:      client,
		Config:      config,
		forwardDone: make(chan struct{}),
	}, nil
}

// RunCommand executes a command on the remote and returns output.
func (s *SSHSession) RunCommand(cmd string) (string, error) {
	session, err := s.Client.NewSession()
	if err != nil {
		return "", fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	out, err := session.CombinedOutput(cmd)
	return string(out), err
}

// SCPBinary copies the current executable to the remote path.
func (s *SSHSession) SCPBinary(remotePath string) error {
	localBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable: %w", err)
	}

	localFile, err := os.Open(localBin)
	if err != nil {
		return fmt.Errorf("open binary: %w", err)
	}
	defer localFile.Close()

	stat, err := localFile.Stat()
	if err != nil {
		return fmt.Errorf("stat binary: %w", err)
	}

	// Ensure remote directory exists
	remoteDir := filepath.Dir(remotePath)
	s.RunCommand(fmt.Sprintf("mkdir -p %s", remoteDir))

	session, err := s.Client.NewSession()
	if err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	// Use SCP protocol
	go func() {
		w, _ := session.StdinPipe()
		defer w.Close()
		fmt.Fprintf(w, "C0755 %d %s\n", stat.Size(), filepath.Base(remotePath))
		io.Copy(w, localFile)
		fmt.Fprint(w, "\x00")
	}()

	if err := session.Run(fmt.Sprintf("scp -t %s", remotePath)); err != nil {
		return fmt.Errorf("scp: %w", err)
	}

	return nil
}

// StartRemoteAgent starts the agent process on the remote machine.
func (s *SSHSession) StartRemoteAgent(port int) error {
	// Check if already running
	out, err := s.RunCommand("pgrep -f 'gpusched --agent'")
	if err == nil && strings.TrimSpace(out) != "" {
		fmt.Printf("Remote agent already running (PID %s)\n", strings.TrimSpace(out))
		return nil
	}

	// Start agent
	cmd := fmt.Sprintf("nohup ~/.gpusched/gpusched --agent --port %d > ~/.gpusched/agent.log 2>&1 &", port)
	if _, err := s.RunCommand(cmd); err != nil {
		return fmt.Errorf("start remote agent: %w", err)
	}

	// Brief wait then verify
	time.Sleep(time.Second)
	verifyCmd := fmt.Sprintf("curl -s localhost:%d/topology > /dev/null 2>&1 && echo ok", port)
	out, err = s.RunCommand(verifyCmd)
	if err != nil || !strings.Contains(out, "ok") {
		return fmt.Errorf("remote agent failed to start (check ~/.gpusched/agent.log)")
	}

	fmt.Printf("Remote agent started on port %d\n", port)
	return nil
}

// ForwardPort sets up local port forwarding: local random port -> remote port.
// Returns the local port number.
func (s *SSHSession) ForwardPort(remotePort int) (int, error) {
	// Listen on random local port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("listen: %w", err)
	}

	localPort := listener.Addr().(*net.TCPAddr).Port
	s.LocalPort = localPort

	go func() {
		defer listener.Close()
		for {
			local, err := listener.Accept()
			if err != nil {
				select {
				case <-s.forwardDone:
					return
				default:
					continue
				}
			}

			remoteAddr := fmt.Sprintf("localhost:%d", remotePort)
			remote, err := s.Client.Dial("tcp", remoteAddr)
			if err != nil {
				local.Close()
				continue
			}

			go func() {
				defer local.Close()
				defer remote.Close()
				done := make(chan struct{}, 2)
				go func() { io.Copy(remote, local); done <- struct{}{} }()
				go func() { io.Copy(local, remote); done <- struct{}{} }()
				<-done
			}()
		}
	}()

	fmt.Printf("Port forward: localhost:%d -> remote:%d\n", localPort, remotePort)
	return localPort, nil
}

// Close terminates the SSH session.
func (s *SSHSession) Close() error {
	close(s.forwardDone)
	return s.Client.Close()
}

func loadPrivateKey(keyPath string) (ssh.Signer, error) {
	if keyPath == "" {
		// Try default key locations
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home: %w", err)
		}
		candidates := []string{
			filepath.Join(home, ".ssh", "id_ed25519"),
			filepath.Join(home, ".ssh", "id_rsa"),
		}
		for _, c := range candidates {
			if key, err := readKey(c); err == nil {
				return key, nil
			}
		}
		return nil, fmt.Errorf("no SSH key found (tried ~/.ssh/id_ed25519, ~/.ssh/id_rsa)")
	}

	// Expand ~ in path
	if strings.HasPrefix(keyPath, "~/") {
		home, _ := os.UserHomeDir()
		keyPath = filepath.Join(home, keyPath[2:])
	}

	return readKey(keyPath)
}

func readKey(path string) (ssh.Signer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ssh.ParsePrivateKey(data)
}
