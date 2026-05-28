//go:build mock && integration

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

func listenRandom() (net.Listener, error) {
	return net.Listen("tcp", "127.0.0.1:0")
}

// TestFullClusterWithRemote tests: local agent + remote SSH agent, scheduler picks best node.
// Run with: go test -tags "mock integration" -v -timeout 60s ./internal/cluster/ -run TestFullClusterWithRemote
func TestFullClusterWithRemote(t *testing.T) {
	// --- Start local agent (8 GPUs) ---
	os.Setenv("GPUSCHED_MOCK_GPUS", "8")
	localSrv, err := agentserver.NewAgentServer("")
	if err != nil {
		t.Fatalf("local agent: %v", err)
	}
	os.Unsetenv("GPUSCHED_MOCK_GPUS")

	localLn, err := listenRandom()
	if err != nil {
		t.Fatal(err)
	}
	localPort := localLn.Addr().(*net.TCPAddr).Port
	go http.Serve(localLn, localSrv.Handler())
	defer localLn.Close()

	localClient := NewAgentClient(fmt.Sprintf("http://127.0.0.1:%d", localPort))

	// --- Connect to remote agent (4 GPUs on SSH server) ---
	config := &SSHConfig{
		User:    "root",
		Host:    "203.0.113.10",
		Port:    22,
		KeyPath: os.Getenv("HOME") + "/.ssh/host",
	}

	session, err := Connect(*config)
	if err != nil {
		t.Fatalf("SSH connect: %v", err)
	}
	defer session.Close()

	// Start remote agent
	session.RunCommand("pkill -f 'gpusched --agent' 2>/dev/null")
	time.Sleep(500 * time.Millisecond)
	if err := session.StartRemoteAgent(9712); err != nil {
		t.Fatalf("start remote agent: %v", err)
	}

	remoteLocalPort, err := session.ForwardPort(9712)
	if err != nil {
		t.Fatalf("port forward: %v", err)
	}
	remoteClient := NewAgentClient(fmt.Sprintf("http://localhost:%d", remoteLocalPort))

	// --- Build cluster ---
	mgr := NewNodeManager()
	cs := NewClusterScheduler(mgr)

	localNode, err := mgr.AddRemoteNode("local-8gpu", "local", localClient)
	if err != nil {
		t.Fatalf("add local node: %v", err)
	}
	t.Logf("Local node: %d GPUs", len(localNode.Devices))

	remoteNode, err := mgr.AddRemoteNode("remote-4gpu", "ssh-server", remoteClient)
	if err != nil {
		t.Fatalf("add remote node: %v", err)
	}
	t.Logf("Remote node: %d GPUs", len(remoteNode.Devices))

	cs.StartLoop()
	defer cs.Stop()

	// --- Test 1: Small job (2 GPUs) — should go to node with best score ---
	t.Log("=== Test 1: 2-GPU job ===")
	job1 := &scheduler.Job{
		ID:        "cluster-test-1",
		Command:   "echo test1 && hostname",
		NumGPUs:   2,
		MinVRAMMB: 0,
		Priority:  10,
	}
	cs.SubmitJob(job1)

	time.Sleep(3 * time.Second)
	completed := cs.CompletedJobs()
	if len(completed) == 0 {
		running := cs.RunningJobs()
		for id, cj := range running {
			t.Logf("still running: %s on %s", id, cj.NodeID)
		}
		t.Fatal("job1 never completed")
	}
	t.Logf("Job1 dispatched to: %s, GPUs: %v", completed[0].NodeID, completed[0].GPUIDs)

	// --- Test 2: Large job (6 GPUs) — must go to 8-GPU local node ---
	t.Log("=== Test 2: 6-GPU job (must go to 8-GPU node) ===")
	job2 := &scheduler.Job{
		ID:        "cluster-test-2",
		Command:   "echo need6gpus && hostname",
		NumGPUs:   6,
		MinVRAMMB: 0,
		Priority:  5,
	}
	cs.SubmitJob(job2)

	time.Sleep(3 * time.Second)
	completed = cs.CompletedJobs()
	var job2Result *ClusterJob
	for _, cj := range completed {
		if cj.ID == "cluster-test-2" {
			job2Result = cj
			break
		}
	}
	if job2Result == nil {
		t.Fatal("job2 never completed")
	}
	if job2Result.NodeID != "local-8gpu" {
		t.Errorf("expected 6-GPU job on local-8gpu, got %s", job2Result.NodeID)
	}
	t.Logf("Job2 dispatched to: %s, GPUs: %v", job2Result.NodeID, job2Result.GPUIDs)

	// --- Test 3: Read logs from remote job ---
	t.Log("=== Test 3: Read remote job logs ===")
	// Find which job went to remote
	for _, cj := range completed {
		if cj.NodeID == "remote-4gpu" {
			logs, err := remoteClient.GetLogs(cj.ID, 0)
			if err != nil {
				t.Errorf("get logs: %v", err)
			} else {
				t.Logf("Remote job %s logs: %s", cj.ID, logs.Data)
			}
		}
	}

	t.Log("Full cluster integration test PASSED!")
}
