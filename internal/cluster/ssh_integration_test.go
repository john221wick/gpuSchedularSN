//go:build mock && integration

package cluster

import (
	"fmt"
	"os"
	"testing"
)

// Run with: go test -tags "mock integration" -v -timeout 60s ./internal/cluster/ -run TestSSHIntegration
func TestSSHIntegration(t *testing.T) {
	config := &SSHConfig{
		User:    "root",
		Host:    "203.0.113.10",
		Port:    22,
		KeyPath: os.Getenv("HOME") + "/.ssh/host",
	}

	t.Logf("Connecting to %s@%s:%d...", config.User, config.Host, config.Port)
	session, err := Connect(*config)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer session.Close()
	t.Log("SSH connected!")

	// Run a test command
	out, err := session.RunCommand("hostname && uname -m")
	if err != nil {
		t.Fatalf("RunCommand failed: %v", err)
	}
	t.Logf("Remote: %s", out)

	// Start remote agent (binary already deployed via scp)
	t.Log("Starting remote agent...")
	err = session.StartRemoteAgent(9712)
	if err != nil {
		t.Fatalf("StartRemoteAgent failed: %v", err)
	}

	// Set up port forward
	t.Log("Setting up port forward...")
	localPort, err := session.ForwardPort(9712)
	if err != nil {
		t.Fatalf("ForwardPort failed: %v", err)
	}
	t.Logf("Port forward: localhost:%d -> remote:9712", localPort)

	// Connect agent client through tunnel
	client := NewAgentClient(fmt.Sprintf("http://localhost:%d", localPort))

	// Get topology
	topo, err := client.GetTopology()
	if err != nil {
		t.Fatalf("GetTopology failed: %v", err)
	}
	t.Logf("Remote GPUs: %d", len(topo.Devices))
	if len(topo.Devices) != 4 {
		t.Errorf("expected 4 mock GPUs, got %d", len(topo.Devices))
	}

	// Create cluster with remote node
	mgr := NewNodeManager()
	node, err := mgr.AddRemoteNode("ssh-test", config.Host, client)
	if err != nil {
		t.Fatalf("AddRemoteNode failed: %v", err)
	}
	t.Logf("Node added: %s with %d GPUs", node.Name, len(node.Devices))

	t.Log("Full SSH pipeline test PASSED!")
}
