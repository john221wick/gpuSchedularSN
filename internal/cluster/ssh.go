package cluster

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
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
	if len(parts) == 2 {
		config.User = parts[0]
		config.Host = parts[1]
	} else {
		// Bare hostname or SSH config alias (e.g. "ssh host")
		// Try resolving from ~/.ssh/config, fall back to alias as-is
		resolved := resolveSSHAlias(userHost)
		config.Host = resolved.Host
		if resolved.User != "" {
			config.User = resolved.User
		}
		if resolved.Port != 0 {
			config.Port = resolved.Port
		}
		if resolved.KeyPath != "" && config.KeyPath == "" {
			config.KeyPath = resolved.KeyPath
		}
	}

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

// RemoteInfo holds auto-detected info about remote machine.
type RemoteInfo struct {
	Hostname  string // machine hostname
	Arch      string // x86_64, aarch64
	GoArch    string // amd64, arm64
	GPUVendor string // nvidia, amd, intel, none
	GPUCount  int
	GPUName   string // first GPU name detected
	OS        string // e.g. "Ubuntu 22.04"
}

// DetectRemote auto-detects remote machine capabilities.
func (s *SSHSession) DetectRemote() (*RemoteInfo, error) {
	info := &RemoteInfo{}

	// Hostname
	hostnameOut, _ := s.RunCommand("hostname")
	info.Hostname = strings.TrimSpace(hostnameOut)

	// Arch
	archOut, err := s.RunCommand("uname -m")
	if err != nil {
		return nil, fmt.Errorf("detect arch: %w", err)
	}
	info.Arch = strings.TrimSpace(archOut)
	switch info.Arch {
	case "x86_64":
		info.GoArch = "amd64"
	case "aarch64", "arm64":
		info.GoArch = "arm64"
	default:
		return nil, fmt.Errorf("unsupported arch: %s", info.Arch)
	}

	// OS
	osOut, _ := s.RunCommand("cat /etc/os-release 2>/dev/null | grep PRETTY_NAME | cut -d'\"' -f2")
	info.OS = strings.TrimSpace(osOut)
	if info.OS == "" {
		info.OS = "Linux"
	}

	// GPU detection: try nvidia first, then amd, then intel
	if out, err := s.RunCommand("nvidia-smi --query-gpu=count,name --format=csv,noheader,nounits 2>/dev/null | head -1"); err == nil && strings.TrimSpace(out) != "" {
		info.GPUVendor = "nvidia"
		parts := strings.SplitN(strings.TrimSpace(out), ", ", 2)
		if len(parts) >= 1 {
			if n, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil {
				info.GPUCount = n
			}
		}
		if len(parts) >= 2 {
			info.GPUName = strings.TrimSpace(parts[1])
		}
		// If count query didn't work, count lines
		if info.GPUCount == 0 {
			if countOut, err := s.RunCommand("nvidia-smi --query-gpu=name --format=csv,noheader 2>/dev/null | wc -l"); err == nil {
				if n, err := strconv.Atoi(strings.TrimSpace(countOut)); err == nil {
					info.GPUCount = n
				}
			}
		}
	} else if out, err := s.RunCommand("rocm-smi --showproductname 2>/dev/null | grep 'GPU' | head -1"); err == nil && strings.TrimSpace(out) != "" {
		info.GPUVendor = "amd"
		info.GPUName = strings.TrimSpace(out)
		if countOut, err := s.RunCommand("rocm-smi --showid 2>/dev/null | grep 'GPU' | wc -l"); err == nil {
			if n, err := strconv.Atoi(strings.TrimSpace(countOut)); err == nil {
				info.GPUCount = n
			}
		}
	} else if out, err := s.RunCommand("xpu-smi discovery 2>/dev/null | grep 'Device Name' | head -1"); err == nil && strings.TrimSpace(out) != "" {
		info.GPUVendor = "intel"
		info.GPUName = strings.TrimSpace(out)
	} else {
		info.GPUVendor = "none"
	}

	fmt.Printf("Remote: arch=%s os=%s gpu=%s (%d x %s)\n",
		info.Arch, info.OS, info.GPUVendor, info.GPUCount, info.GPUName)

	return info, nil
}

// CrossCompileAndSCP builds for the remote arch and copies to remotePath.
// Uses -tags smi (real GPUs via nvidia-smi) or -tags mock (fake GPUs).
// Both avoid CGO so cross-compilation macOS→Linux works.
func (s *SSHSession) CrossCompileAndSCP(remotePath string, mock bool) error {
	info, err := s.DetectRemote()
	if err != nil {
		return err
	}

	// Kill any existing agent — we're deploying fresh on each connect
	s.RunCommand("pkill -f 'gpusched --agent' 2>/dev/null || true")
	time.Sleep(500 * time.Millisecond)

	// Find project root
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable: %w", err)
	}
	projectRoot := findProjectRoot(execPath)
	if projectRoot == "" {
		if cwd, err := os.Getwd(); err == nil {
			projectRoot = findProjectRoot(cwd)
		}
	}
	if projectRoot == "" {
		return fmt.Errorf("could not find project root (go.mod)")
	}

	// Use smi tag for real GPUs (nvidia-smi based), mock for testing
	// Both avoid CGO so cross-compile works
	tag := "smi"
	if mock {
		tag = "mock"
	}
	tmpBin := filepath.Join(os.TempDir(), fmt.Sprintf("gpusched-linux-%s", info.GoArch))
	defer os.Remove(tmpBin)

	args := []string{"build", "-o", tmpBin, "-tags", tag, "./cmd/"}

	buildCmd := exec.Command("go", args...)
	buildCmd.Dir = projectRoot
	buildCmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+info.GoArch, "CGO_ENABLED=0")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("cross-compile failed: %s: %w", string(out), err)
	}

	fmt.Printf("Cross-compiled: %s (arch=%s)\n", tmpBin, info.GoArch)

	// Remove old binary and deploy new one
	s.RunCommand(fmt.Sprintf("rm -f $(eval echo %s)", remotePath))
	return s.SCPFile(tmpBin, remotePath)
}

// SCPFile copies a local file to the remote path.
// Uses cat-based transfer instead of scp protocol for reliability.
func (s *SSHSession) SCPFile(localPath, remotePath string) error {
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", localPath, err)
	}
	defer localFile.Close()

	// Resolve ~ and ensure directory exists on remote
	// Use eval to expand tilde in shell
	resolvedOut, _ := s.RunCommand(fmt.Sprintf("eval echo %s", remotePath))
	resolved := strings.TrimSpace(resolvedOut)
	if resolved == "" {
		resolved = remotePath
	}

	s.RunCommand(fmt.Sprintf("mkdir -p $(dirname %s)", resolved))

	session, err := s.Client.NewSession()
	if err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	// Use cat > file approach — more reliable than scp -t with tilde paths
	go func() {
		w, _ := session.StdinPipe()
		defer w.Close()
		io.Copy(w, localFile)
	}()

	cmd := fmt.Sprintf("cat > %s && chmod 755 %s", resolved, resolved)
	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("transfer: %w", err)
	}

	return nil
}

// findProjectRoot walks up from dir looking for go.mod.
func findProjectRoot(start string) string {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// StartRemoteAgent starts the agent process on the remote machine.
// baseDir is the working directory (e.g. ~/gpuschedular) where binary, logs, state live.
// gpuCount is the detected real GPU count — passed as GPUSCHED_MOCK_GPUS env to the mock agent.
// Uses a pidfile at baseDir/agent.pid to track the process.
//
// Startup is hardened against the common failure modes:
//  1. Pre-clean: kill any stale agent, free the port, drop the stale pidfile.
//  2. Poll the health endpoint for up to 20s — restoring many persisted jobs can
//     delay the listen well past 1s, so a single one-shot check races the bind.
//  3. Fallback: if it still won't come up, a corrupt/oversized state.json stalling
//     restore is the usual culprit — wipe it and retry once from a clean slate.
func (s *SSHSession) StartRemoteAgent(port int, baseDir string, gpuCount int) error {
	gpuEnv := ""
	if gpuCount > 0 {
		gpuEnv = fmt.Sprintf("export GPUSCHED_MOCK_GPUS=%d; ", gpuCount)
	}

	// attempt boots the agent once and polls until healthy or timeout.
	attempt := func() error {
		s.preStartCleanup(port, baseDir)

		pidFile := baseDir + "/agent.pid"
		cmd := fmt.Sprintf("%snohup %s/gpusched --agent --port %d --dir %s > %s/agent.log 2>&1 & echo $! > %s",
			gpuEnv, baseDir, port, baseDir, baseDir, pidFile)
		if _, err := s.RunCommand(cmd); err != nil {
			return fmt.Errorf("start remote agent: %w", err)
		}
		return s.waitAgentHealthy(port, 20*time.Second)
	}

	if err := attempt(); err != nil {
		// Fallback: stale/corrupt state.json is the usual cause of a stalled restore.
		// Wipe it and retry once from a clean slate.
		s.RunCommand(fmt.Sprintf("rm -f %s/state.json %s/state.json.tmp", baseDir, baseDir))
		if err2 := attempt(); err2 != nil {
			logOut, _ := s.RunCommand(fmt.Sprintf("cat %s/agent.log 2>/dev/null | tail -30", baseDir))
			return fmt.Errorf("remote agent failed to start (after state reset):\n%s", logOut)
		}
		fmt.Println("Remote agent recovered after state reset")
	}

	fmt.Printf("Remote agent started on port %d (dir: %s)\n", port, baseDir)
	return nil
}

// preStartCleanup kills any stale agent and frees the port before a fresh start.
// Every step is best-effort (|| true) — the remote may lack any given tool.
func (s *SSHSession) preStartCleanup(port int, baseDir string) {
	pidFile := baseDir + "/agent.pid"
	// Kill by recorded pid, by command match, and anything still holding the port.
	s.RunCommand(fmt.Sprintf("kill $(cat %s 2>/dev/null) 2>/dev/null || true", pidFile))
	s.RunCommand("pkill -f 'gpusched --agent' 2>/dev/null || true")
	s.RunCommand(fmt.Sprintf("fuser -k %d/tcp 2>/dev/null || true", port))
	s.RunCommand(fmt.Sprintf("rm -f %s", pidFile))
	// Give the OS a moment to release the listening socket.
	time.Sleep(500 * time.Millisecond)
}

// waitAgentHealthy polls the agent /topology endpoint until it responds or timeout.
// Probe tries curl, then wget, then a bash /dev/tcp connect — so it works even on
// minimal images that ship none of the usual HTTP clients.
func (s *SSHSession) waitAgentHealthy(port int, timeout time.Duration) error {
	probe := fmt.Sprintf(
		"curl -s -m 2 localhost:%d/topology >/dev/null 2>&1 && echo ok || "+
			"wget -q -T 2 -O /dev/null localhost:%d/topology 2>/dev/null && echo ok || "+
			"(exec 3<>/dev/tcp/localhost/%d && echo ok) 2>/dev/null",
		port, port, port)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if out, _ := s.RunCommand(probe); strings.Contains(out, "ok") {
			return nil
		}
		time.Sleep(400 * time.Millisecond)
	}
	return fmt.Errorf("health check timed out after %s", timeout)
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

// resolveSSHAlias parses ~/.ssh/config to resolve a Host alias.
// Returns partial SSHConfig with whatever fields are found.
func resolveSSHAlias(alias string) SSHConfig {
	result := SSHConfig{Host: alias} // default: alias is the hostname

	home, err := os.UserHomeDir()
	if err != nil {
		return result
	}

	data, err := os.ReadFile(filepath.Join(home, ".ssh", "config"))
	if err != nil {
		return result
	}

	lines := strings.Split(string(data), "\n")
	inBlock := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split on first whitespace or =
		var key, val string
		if idx := strings.IndexAny(line, " \t="); idx > 0 {
			key = strings.TrimSpace(line[:idx])
			val = strings.TrimSpace(strings.TrimLeft(line[idx:], " \t="))
		} else {
			continue
		}

		if strings.EqualFold(key, "Host") {
			// Check if this block matches our alias
			// Host can have multiple patterns separated by spaces
			patterns := strings.Fields(val)
			inBlock = false
			for _, p := range patterns {
				if p == alias {
					inBlock = true
					break
				}
			}
			continue
		}

		if !inBlock {
			continue
		}

		switch strings.ToLower(key) {
		case "hostname":
			result.Host = val
		case "user":
			result.User = val
		case "port":
			if p, err := strconv.Atoi(val); err == nil {
				result.Port = p
			}
		case "identityfile":
			// Expand ~
			if strings.HasPrefix(val, "~/") {
				val = filepath.Join(home, val[2:])
			}
			result.KeyPath = val
		}
	}

	return result
}
