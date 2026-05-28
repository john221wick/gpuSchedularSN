package agentserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/john221wick/gpuSchedularSN/internal/agent"
)

type AgentServer struct {
	httpServer *http.Server
	pm         *ProcessManager
	listener   net.Listener
	dataDir    string
}

func NewAgentServer() (*AgentServer, error) {
	count, err := agent.Init()
	if err != nil {
		return nil, fmt.Errorf("agent init: %w", err)
	}
	fmt.Printf("Agent: detected %d GPUs\n", count)

	// Determine vendor from first device
	devices := agent.GetDevices()
	vendor := agent.NVIDIA
	if len(devices) > 0 {
		vendor = devices[0].Vendor
	}

	// Set up directories
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}
	dataDir := filepath.Join(home, ".gpusched")
	logDir := filepath.Join(dataDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("create dirs: %w", err)
	}

	pm := NewProcessManager(logDir, vendor)

	// Restore persisted state and reconcile PIDs
	persisted, err := LoadState(dataDir)
	if err != nil {
		fmt.Printf("Warning: could not load state: %v\n", err)
	} else if len(persisted.Jobs) > 0 {
		pm.RestoreFromState(persisted.Jobs)
		pm.ReconcilePIDs()
		fmt.Printf("Agent: restored %d jobs from state\n", len(persisted.Jobs))
	}

	// Cache topology at creation time (important: mock agents in same process share global state)
	topoResp := &TopologyResponse{
		Devices: agent.GetDevices(),
		Links:   agent.GetTopology(),
	}

	handlers := NewHandlers(pm, topoResp)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /topology", handlers.GetTopology)
	mux.HandleFunc("POST /jobs", handlers.PostJob)
	mux.HandleFunc("GET /status", handlers.GetStatus)
	mux.HandleFunc("GET /logs/{id}", handlers.GetLogs)
	mux.HandleFunc("DELETE /jobs/{id}", handlers.DeleteJob)

	srv := &AgentServer{
		httpServer: &http.Server{Handler: mux},
		pm:         pm,
		dataDir:    dataDir,
	}

	return srv, nil
}

func (s *AgentServer) ListenAndServe(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.listener = ln
	fmt.Printf("Agent server listening on %s\n", addr)
	return s.httpServer.Serve(ln)
}

func (s *AgentServer) ListenAndServeUnix(path string) error {
	// Remove stale socket file
	os.Remove(path)

	ln, err := net.Listen("unix", path)
	if err != nil {
		return err
	}
	s.listener = ln
	fmt.Printf("Agent server listening on unix:%s\n", path)
	return s.httpServer.Serve(ln)
}

// Handler returns the HTTP handler for use in tests.
func (s *AgentServer) Handler() http.Handler {
	return s.httpServer.Handler
}

func (s *AgentServer) Shutdown(ctx context.Context) error {
	// Save state before shutting down
	state := s.pm.ToPersistedState()
	if err := SaveState(s.dataDir, state); err != nil {
		fmt.Printf("Warning: could not save state: %v\n", err)
	}

	agent.Shutdown()
	return s.httpServer.Shutdown(ctx)
}
