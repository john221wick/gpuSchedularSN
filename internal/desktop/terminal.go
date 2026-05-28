package desktop

import (
	"encoding/base64"
	"fmt"
	"io"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// termSession holds one interactive PTY session over SSH.
type termSession struct {
	session *ssh.Session
	stdin   io.WriteCloser
	nodeID  string
}

var (
	ptyMu       sync.Mutex
	ptySessions = make(map[string]*termSession)
)

// StartTerminalSession opens an interactive SSH shell with PTY on the given node.
// Returns a session ID used for all subsequent terminal operations.
func (a *App) StartTerminalSession(nodeID string, cols int, rows int) (string, error) {
	sess, ok := a.sshSessions[nodeID]
	if !ok {
		return "", fmt.Errorf("no SSH session for node %s", nodeID)
	}

	sshSession, err := sess.Client.NewSession()
	if err != nil {
		return "", fmt.Errorf("new SSH session: %w", err)
	}

	// Request PTY
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := sshSession.RequestPty("xterm-256color", rows, cols, modes); err != nil {
		sshSession.Close()
		return "", fmt.Errorf("request PTY: %w", err)
	}

	stdin, err := sshSession.StdinPipe()
	if err != nil {
		sshSession.Close()
		return "", fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := sshSession.StdoutPipe()
	if err != nil {
		sshSession.Close()
		return "", fmt.Errorf("stdout pipe: %w", err)
	}

	stderr, err := sshSession.StderrPipe()
	if err != nil {
		sshSession.Close()
		return "", fmt.Errorf("stderr pipe: %w", err)
	}

	if err := sshSession.Shell(); err != nil {
		sshSession.Close()
		return "", fmt.Errorf("start shell: %w", err)
	}

	sessionID := fmt.Sprintf("pty-%s-%d", nodeID, time.Now().UnixNano())

	ts := &termSession{
		session: sshSession,
		stdin:   stdin,
		nodeID:  nodeID,
	}

	ptyMu.Lock()
	ptySessions[sessionID] = ts
	ptyMu.Unlock()

	outputEvent := "terminal:output:" + sessionID
	exitEvent := "terminal:exit:" + sessionID

	// Read stdout
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				encoded := base64.StdEncoding.EncodeToString(buf[:n])
				wailsRuntime.EventsEmit(a.ctx, outputEvent, encoded)
			}
			if err != nil {
				wailsRuntime.EventsEmit(a.ctx, exitEvent, "disconnected")
				cleanupPTY(sessionID)
				return
			}
		}
	}()

	// Read stderr (merge into same output stream)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				encoded := base64.StdEncoding.EncodeToString(buf[:n])
				wailsRuntime.EventsEmit(a.ctx, outputEvent, encoded)
			}
			if err != nil {
				return
			}
		}
	}()

	return sessionID, nil
}

// WriteTerminalInput sends keystrokes to the PTY session.
// Data is base64-encoded to safely transport control characters.
func (a *App) WriteTerminalInput(sessionID string, data string) error {
	ptyMu.Lock()
	ts, ok := ptySessions[sessionID]
	ptyMu.Unlock()
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return fmt.Errorf("decode input: %w", err)
	}

	_, err = ts.stdin.Write(decoded)
	return err
}

// ResizeTerminal sends a window change request to the PTY.
func (a *App) ResizeTerminal(sessionID string, cols int, rows int) error {
	ptyMu.Lock()
	ts, ok := ptySessions[sessionID]
	ptyMu.Unlock()
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	return ts.session.WindowChange(rows, cols)
}

// StopTerminalSession closes a PTY session.
func (a *App) StopTerminalSession(sessionID string) error {
	return cleanupPTY(sessionID)
}

func cleanupPTY(sessionID string) error {
	ptyMu.Lock()
	ts, ok := ptySessions[sessionID]
	if ok {
		delete(ptySessions, sessionID)
	}
	ptyMu.Unlock()

	if !ok {
		return nil
	}

	ts.stdin.Close()
	return ts.session.Close()
}
