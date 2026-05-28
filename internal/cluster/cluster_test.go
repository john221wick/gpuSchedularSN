//go:build mock

package cluster

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/john221wick/gpuSchedularSN/internal/agentserver"
	"github.com/john221wick/gpuSchedularSN/internal/scheduler"
)

// startTestAgent starts an agent server on a random port, returns URL and cleanup func.
func startTestAgent(t *testing.T, gpuCount int) (string, func()) {
	t.Helper()

	os.Setenv("GPUSCHED_MOCK_GPUS", fmt.Sprintf("%d", gpuCount))
	defer os.Unsetenv("GPUSCHED_MOCK_GPUS")

	srv, err := agentserver.NewAgentServer("")
	if err != nil {
		t.Fatalf("NewAgentServer: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	url := fmt.Sprintf("http://127.0.0.1:%d", port)

	go http.Serve(ln, srv.Handler())

	// Wait for server to be ready
	for i := 0; i < 20; i++ {
		client := NewAgentClient(url)
		if err := client.Ping(); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	return url, func() {
		ln.Close()
	}
}

func TestParseSSHCommand(t *testing.T) {
	tests := []struct {
		input    string
		user     string
		host     string
		port     int
		wantErr  bool
	}{
		{"ssh -p 20544 root@203.0.113.10", "root", "203.0.113.10", 20544, false},
		{"ssh root@192.168.1.100", "root", "192.168.1.100", 22, false},
		{"ssh -p 41922 root@ssh5.vast.ai -L 8080:localhost:8080", "root", "ssh5.vast.ai", 41922, false},
		{"-p 9999 user@example.com", "user", "example.com", 9999, false},
		{"ssh -i /tmp/key -p 2222 ubuntu@10.0.0.1", "ubuntu", "10.0.0.1", 2222, false},
		{"", "", "", 0, true},
		{"ssh", "", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			cfg, err := ParseSSHCommand(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.User != tt.user {
				t.Errorf("user: got %q, want %q", cfg.User, tt.user)
			}
			if cfg.Host != tt.host {
				t.Errorf("host: got %q, want %q", cfg.Host, tt.host)
			}
			if cfg.Port != tt.port {
				t.Errorf("port: got %d, want %d", cfg.Port, tt.port)
			}
		})
	}
}

func TestParseSSHCommandAlias(t *testing.T) {
	// Bare alias should not error — resolves via ~/.ssh/config or uses alias as hostname
	cfg, err := ParseSSHCommand("ssh myalias")
	if err != nil {
		t.Fatalf("bare alias should not error: %v", err)
	}
	// Host should be at minimum the alias itself (if not in ssh config)
	if cfg.Host == "" {
		t.Error("host should not be empty")
	}
	t.Logf("alias resolved: host=%s user=%s port=%d key=%s", cfg.Host, cfg.User, cfg.Port, cfg.KeyPath)
}

func TestAgentClientAllEndpoints(t *testing.T) {
	url, cleanup := startTestAgent(t, 4)
	defer cleanup()

	client := NewAgentClient(url)

	// GetTopology
	topo, err := client.GetTopology()
	if err != nil {
		t.Fatalf("GetTopology: %v", err)
	}
	if len(topo.Devices) != 4 {
		t.Fatalf("expected 4 devices, got %d", len(topo.Devices))
	}

	// PostJob
	spec := agentserver.JobSpec{
		ID:      "test-client-1",
		Command: "echo hello && sleep 1",
		GPUIDs:  []int{0, 1},
	}
	if err := client.PostJob(spec); err != nil {
		t.Fatalf("PostJob: %v", err)
	}

	// GetStatus
	status, err := client.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if len(status.Jobs) == 0 {
		t.Fatal("expected at least 1 job")
	}

	// GetLogs
	time.Sleep(500 * time.Millisecond)
	chunk, err := client.GetLogs("test-client-1", 0)
	if err != nil {
		t.Fatalf("GetLogs: %v", err)
	}
	if chunk.Data == "" {
		t.Log("warning: log data empty (might be timing)")
	}

	// DeleteJob — start a new long-running job to kill
	spec2 := agentserver.JobSpec{ID: "test-client-2", Command: "sleep 60", GPUIDs: []int{2}}
	client.PostJob(spec2)
	time.Sleep(200 * time.Millisecond)
	if err := client.DeleteJob("test-client-2"); err != nil {
		t.Fatalf("DeleteJob: %v", err)
	}

	// Ping
	if err := client.Ping(); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestNodeManagerAndClusterScheduler(t *testing.T) {
	// Start 2 agents with different GPU counts
	url1, cleanup1 := startTestAgent(t, 4)
	defer cleanup1()
	url2, cleanup2 := startTestAgent(t, 8)
	defer cleanup2()

	client1 := NewAgentClient(url1)
	client2 := NewAgentClient(url2)

	// Create manager + scheduler
	mgr := NewNodeManager()
	cs := NewClusterScheduler(mgr)

	// Add nodes
	node1, err := mgr.AddRemoteNode("node-a", "4gpu-node", client1)
	if err != nil {
		t.Fatalf("AddRemoteNode A: %v", err)
	}
	if len(node1.Devices) != 4 {
		t.Fatalf("node A: expected 4 GPUs, got %d", len(node1.Devices))
	}

	node2, err := mgr.AddRemoteNode("node-b", "8gpu-node", client2)
	if err != nil {
		t.Fatalf("AddRemoteNode B: %v", err)
	}
	if len(node2.Devices) != 8 {
		t.Fatalf("node B: expected 8 GPUs, got %d", len(node2.Devices))
	}

	// Verify connected nodes
	connected := mgr.ConnectedNodes()
	if len(connected) != 2 {
		t.Fatalf("expected 2 connected nodes, got %d", len(connected))
	}

	// Start scheduler and submit a job needing 2 GPUs
	cs.StartLoop()
	defer cs.Stop()

	job := &scheduler.Job{
		ID:        "cluster-job-1",
		Command:   "echo cluster test",
		NumGPUs:   2,
		MinVRAMMB: 0,
		Priority:  10,
	}
	cs.SubmitJob(job)

	// Wait for dispatch
	time.Sleep(3 * time.Second)

	// Check running jobs
	running := cs.RunningJobs()
	completed := cs.CompletedJobs()
	qLen := cs.QueueLen()

	t.Logf("Running: %d, Completed: %d, Queued: %d", len(running), len(completed), qLen)

	// Job should have been dispatched (either running or already completed)
	if len(running)+len(completed) == 0 {
		t.Fatal("job was never dispatched")
	}

	if len(completed) > 0 {
		cj := completed[0]
		t.Logf("Job %s completed on node %s with GPUs %v", cj.ID, cj.NodeID, cj.GPUIDs)
	}
	if len(running) > 0 {
		for id, cj := range running {
			t.Logf("Job %s running on node %s with GPUs %v", id, cj.NodeID, cj.GPUIDs)
		}
	}
}

func TestClusterSchedulerPicksBestNode(t *testing.T) {
	// 4-GPU node vs 8-GPU node. Job needs 6 GPUs — must go to 8-GPU node.
	url1, cleanup1 := startTestAgent(t, 4)
	defer cleanup1()
	url2, cleanup2 := startTestAgent(t, 8)
	defer cleanup2()

	mgr := NewNodeManager()
	cs := NewClusterScheduler(mgr)

	mgr.AddRemoteNode("node-4gpu", "small", NewAgentClient(url1))
	mgr.AddRemoteNode("node-8gpu", "big", NewAgentClient(url2))

	cs.StartLoop()
	defer cs.Stop()

	job := &scheduler.Job{
		ID:        "big-job",
		Command:   "echo need 6 gpus",
		NumGPUs:   6,
		MinVRAMMB: 0,
		Priority:  5,
	}
	cs.SubmitJob(job)

	// Wait for dispatch + completion
	time.Sleep(3 * time.Second)

	completed := cs.CompletedJobs()
	running := cs.RunningJobs()

	var dispatched *ClusterJob
	if len(completed) > 0 {
		dispatched = completed[0]
	}
	for _, cj := range running {
		dispatched = cj
	}

	if dispatched == nil {
		t.Fatal("job was never dispatched")
	}

	if dispatched.NodeID != "node-8gpu" {
		t.Errorf("expected job on node-8gpu, got %s", dispatched.NodeID)
	}
	if len(dispatched.GPUIDs) != 6 {
		t.Errorf("expected 6 GPUs allocated, got %d", len(dispatched.GPUIDs))
	}

	t.Logf("Job dispatched to %s with GPUs %v", dispatched.NodeID, dispatched.GPUIDs)
}
