package cluster

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/john221wick/gpuSchedularSN/internal/scheduler"
)

type NodeManager struct {
	mu      sync.RWMutex
	nodes   map[string]*Node
	clients map[string]AgentClient
	stopCh  chan struct{}

	// Called when a node reconnects with its remote job statuses.
	// Set by ClusterScheduler after construction.
	OnReconnect func(nodeID string)
}

func NewNodeManager() *NodeManager {
	return &NodeManager{
		nodes:   make(map[string]*Node),
		clients: make(map[string]AgentClient),
		stopCh:  make(chan struct{}),
	}
}

func (nm *NodeManager) AddLocalNode(client AgentClient) (*Node, error) {
	hostname, _ := os.Hostname()
	return nm.addNode("local", hostname, client)
}

func (nm *NodeManager) AddRemoteNode(id, name string, client AgentClient) (*Node, error) {
	return nm.addNode(id, name, client)
}

func (nm *NodeManager) addNode(id, name string, client AgentClient) (*Node, error) {
	topo, err := client.GetTopology()
	if err != nil {
		return nil, fmt.Errorf("get topology from %s: %w", name, err)
	}

	topology := scheduler.BuildTopology(topo.Devices, topo.Links)

	node := &Node{
		ID:       id,
		Name:     name,
		AgentURL: "",
		Status:   NodeConnected,
		Devices:  topo.Devices,
		Links:    topo.Links,
		Topology: topology,
	}

	nm.mu.Lock()
	nm.nodes[id] = node
	nm.clients[id] = client
	nm.mu.Unlock()

	fmt.Printf("Node added: %s (%s) — %d GPUs\n", name, id, len(topo.Devices))
	return node, nil
}

func (nm *NodeManager) RemoveNode(nodeID string) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if _, exists := nm.nodes[nodeID]; !exists {
		return fmt.Errorf("node %s not found", nodeID)
	}

	delete(nm.nodes, nodeID)
	delete(nm.clients, nodeID)
	return nil
}

func (nm *NodeManager) GetNode(nodeID string) (*Node, bool) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	node, ok := nm.nodes[nodeID]
	return node, ok
}

func (nm *NodeManager) GetClient(nodeID string) (AgentClient, bool) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	client, ok := nm.clients[nodeID]
	return client, ok
}

func (nm *NodeManager) AllNodes() []*Node {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	nodes := make([]*Node, 0, len(nm.nodes))
	for _, n := range nm.nodes {
		nodes = append(nodes, n)
	}
	return nodes
}

func (nm *NodeManager) ConnectedNodes() []*Node {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	var nodes []*Node
	for _, n := range nm.nodes {
		if n.Status == NodeConnected {
			nodes = append(nodes, n)
		}
	}
	return nodes
}

func (nm *NodeManager) StartHeartbeat() {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-nm.stopCh:
				return
			case <-ticker.C:
				nm.heartbeatAll()
			}
		}
	}()
}

func (nm *NodeManager) heartbeatAll() {
	nm.mu.RLock()
	nodeIDs := make([]string, 0, len(nm.nodes))
	for id := range nm.nodes {
		nodeIDs = append(nodeIDs, id)
	}
	nm.mu.RUnlock()

	for _, id := range nodeIDs {
		nm.mu.RLock()
		client, ok := nm.clients[id]
		node, nodeOk := nm.nodes[id]
		nm.mu.RUnlock()

		if !ok || !nodeOk {
			continue
		}

		err := client.Ping()

		nm.mu.Lock()
		wasDisconnected := node.Status == NodeDisconnected
		if err != nil {
			if node.Status == NodeConnected {
				node.Status = NodeDisconnected
				fmt.Printf("Node %s (%s): disconnected\n", node.Name, id)
			}
		} else {
			if wasDisconnected {
				node.Status = NodeConnected
				fmt.Printf("Node %s (%s): reconnected\n", node.Name, id)
			}
		}
		nm.mu.Unlock()

		// Trigger reconciliation on reconnect
		if wasDisconnected && err == nil && nm.OnReconnect != nil {
			nm.OnReconnect(id)
		}
	}
}

func (nm *NodeManager) Stop() {
	close(nm.stopCh)
}
