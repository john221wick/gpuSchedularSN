package cluster

import (
	"container/heap"
	"fmt"
	"sync"
	"time"

	"github.com/john221wick/gpuSchedularSN/internal/agent"
	"github.com/john221wick/gpuSchedularSN/internal/agentserver"
	"github.com/john221wick/gpuSchedularSN/internal/scheduler"
)

type ClusterScheduler struct {
	mu        sync.Mutex
	manager   *NodeManager
	queue     scheduler.JobQueue
	running   map[string]*ClusterJob           // jobID -> ClusterJob
	allocated map[string]map[int]string         // nodeID -> gpuID -> jobID
	completed []*ClusterJob
	stopCh    chan struct{}

	// TransferFn syncs working dir to remote before job launch.
	// Returns remote work dir path. Nil = skip transfer.
	TransferFn func(localDir, nodeID string, job *ClusterJob) (string, error)
}

func NewClusterScheduler(manager *NodeManager) *ClusterScheduler {
	cs := &ClusterScheduler{
		manager:   manager,
		running:   make(map[string]*ClusterJob),
		allocated: make(map[string]map[int]string),
		stopCh:    make(chan struct{}),
	}

	// Wire reconnect callback
	manager.OnReconnect = func(nodeID string) {
		client, ok := manager.GetClient(nodeID)
		if !ok {
			return
		}
		status, err := client.GetStatus()
		if err != nil {
			fmt.Printf("Reconcile %s: failed to get status: %v\n", nodeID, err)
			return
		}
		cs.Reconcile(nodeID, status.Jobs)
	}

	heap.Init(&cs.queue)
	return cs
}

func (cs *ClusterScheduler) SubmitJob(job *scheduler.Job) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	job.Status = scheduler.Queued
	job.SubmittedAt = time.Now()
	cs.queue.PushJob(job)
	fmt.Printf("Job %s queued (need %d GPUs)\n", job.ID, job.NumGPUs)
}

func (cs *ClusterScheduler) KillJob(jobID string) error {
	cs.mu.Lock()
	cj, exists := cs.running[jobID]
	cs.mu.Unlock()

	if !exists {
		// Try removing from queue
		cs.mu.Lock()
		removed := cs.queue.RemoveByID(jobID)
		cs.mu.Unlock()
		if removed != nil {
			fmt.Printf("Job %s removed from queue\n", jobID)
			return nil
		}
		return fmt.Errorf("job %s not found", jobID)
	}

	client, ok := cs.manager.GetClient(cj.NodeID)
	if !ok {
		return fmt.Errorf("node %s not connected", cj.NodeID)
	}

	if err := client.DeleteJob(jobID); err != nil {
		return fmt.Errorf("kill job on node %s: %w", cj.NodeID, err)
	}

	cs.mu.Lock()
	cs.finishJob(jobID, scheduler.Failed)
	cs.mu.Unlock()

	return nil
}

func (cs *ClusterScheduler) StartLoop() {
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-cs.stopCh:
				return
			case <-ticker.C:
				cs.tick()
			}
		}
	}()
}

func (cs *ClusterScheduler) tick() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// First: poll running jobs for completion
	cs.pollRunningJobs()

	job := cs.queue.PeekJob()
	if job == nil {
		return
	}

	type candidate struct {
		nodeID string
		gpuIDs []int
		score  float32
		free   int
	}

	var best *candidate
	nodes := cs.manager.ConnectedNodes()

	for _, node := range nodes {
		freeGPUIDs := cs.freeGPUsOnNode(node)
		if len(freeGPUIDs) < job.NumGPUs {
			continue
		}

		// Build filtered topology with only free GPUs
		filteredTopo := cs.buildFilteredTopology(node, freeGPUIDs)
		if filteredTopo == nil {
			continue
		}

		scored := scheduler.Score(filteredTopo, job.NumGPUs, job.MinVRAMMB)
		if scored == nil {
			continue
		}

		if best == nil || scored.Score > best.score ||
			(scored.Score == best.score && len(freeGPUIDs) > best.free) {
			best = &candidate{
				nodeID: node.ID,
				gpuIDs: scored.GPUIDs,
				score:  scored.Score,
				free:   len(freeGPUIDs),
			}
		}
	}

	if best == nil {
		return
	}

	// Pop job from queue
	job = cs.queue.PopJob()

	// Resolve paths from node config
	var localDir, remoteDir string
	if node, ok := cs.manager.GetNode(best.nodeID); ok {
		localDir = node.LocalDir
		remoteDir = node.RemoteDir
	}

	cj := &ClusterJob{
		Job:     *job,
		NodeID:  best.nodeID,
		WorkDir: remoteDir,
	}
	cj.Status = scheduler.Running
	cj.StartedAt = time.Now()
	cj.GPUIDs = best.gpuIDs

	// File transfer: rsync node.LocalDir → node.RemoteDir on remote
	if cs.TransferFn != nil && localDir != "" && remoteDir != "" && best.nodeID != "local" {
		resultDir, err := cs.TransferFn(localDir, best.nodeID, cj)
		if err != nil {
			fmt.Printf("Job %s: file transfer failed: %v\n", job.ID, err)
			cj.Status = scheduler.Failed
			cs.completed = append(cs.completed, cj)
			return
		}
		cj.WorkDir = resultDir
	}

	// Dispatch to agent
	client, ok := cs.manager.GetClient(best.nodeID)
	if !ok {
		fmt.Printf("Job %s: node %s lost during dispatch\n", job.ID, best.nodeID)
		cj.Status = scheduler.Failed
		cs.completed = append(cs.completed, cj)
		return
	}

	spec := agentserver.JobSpec{
		ID:      job.ID,
		Command: job.Command,
		GPUIDs:  best.gpuIDs,
		WorkDir: cj.WorkDir,
	}
	if err := client.PostJob(spec); err != nil {
		fmt.Printf("Job %s: dispatch failed: %v\n", job.ID, err)
		cj.Status = scheduler.Failed
		cs.completed = append(cs.completed, cj)
		return
	}

	// Track
	cs.running[job.ID] = cj
	cs.allocateGPUs(best.nodeID, best.gpuIDs, job.ID)

	fmt.Printf("Job %s dispatched to node %s GPUs %v (score: %.0f)\n",
		job.ID, best.nodeID, best.gpuIDs, best.score)
}

func (cs *ClusterScheduler) pollRunningJobs() {
	// Group running jobs by node
	nodeJobs := make(map[string][]string)
	for jobID, cj := range cs.running {
		nodeJobs[cj.NodeID] = append(nodeJobs[cj.NodeID], jobID)
	}

	for nodeID, jobIDs := range nodeJobs {
		client, ok := cs.manager.GetClient(nodeID)
		if !ok {
			continue
		}

		status, err := client.GetStatus()
		if err != nil {
			continue
		}

		// Build lookup
		remoteStatus := make(map[string]string)
		for _, js := range status.Jobs {
			remoteStatus[js.ID] = js.Status
		}

		for _, jobID := range jobIDs {
			rStatus, exists := remoteStatus[jobID]
			if !exists {
				// Job disappeared from remote — mark failed
				cs.finishJob(jobID, scheduler.Failed)
				continue
			}
			switch rStatus {
			case "done":
				cs.finishJob(jobID, scheduler.Done)
			case "failed":
				cs.finishJob(jobID, scheduler.Failed)
			}
		}
	}
}

func (cs *ClusterScheduler) Reconcile(nodeID string, remoteJobs []agentserver.JobStatusResponse) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	remoteStatus := make(map[string]string)
	for _, rj := range remoteJobs {
		remoteStatus[rj.ID] = rj.Status
	}

	for jobID, cj := range cs.running {
		if cj.NodeID != nodeID {
			continue
		}

		rStatus, exists := remoteStatus[jobID]
		if !exists {
			// Agent lost track of job
			fmt.Printf("Reconcile: job %s not found on node %s, marking failed\n", jobID, nodeID)
			cs.finishJob(jobID, scheduler.Failed)
			continue
		}

		switch rStatus {
		case "done":
			fmt.Printf("Reconcile: job %s completed on node %s during disconnect\n", jobID, nodeID)
			cs.finishJob(jobID, scheduler.Done)
		case "failed":
			fmt.Printf("Reconcile: job %s failed on node %s during disconnect\n", jobID, nodeID)
			cs.finishJob(jobID, scheduler.Failed)
		case "running":
			// Still running, no-op
		}
	}
}

func (cs *ClusterScheduler) finishJob(jobID string, status scheduler.JobStatus) {
	cj, exists := cs.running[jobID]
	if !exists {
		return
	}

	cj.Status = status
	cs.freeGPUs(cj.NodeID, cj.GPUIDs)
	cs.completed = append(cs.completed, cj)
	delete(cs.running, jobID)

	fmt.Printf("Job %s %s (node %s, GPUs %v freed)\n", jobID, status.String(), cj.NodeID, cj.GPUIDs)
}

// freeGPUsOnNode returns GPU IDs not allocated on this node.
func (cs *ClusterScheduler) freeGPUsOnNode(node *Node) []int {
	nodeAlloc := cs.allocated[node.ID]
	var free []int
	for _, d := range node.Devices {
		if nodeAlloc == nil || nodeAlloc[d.ID] == "" {
			free = append(free, d.ID)
		}
	}
	return free
}

// buildFilteredTopology creates a topology containing only the given free GPU IDs.
func (cs *ClusterScheduler) buildFilteredTopology(node *Node, freeGPUIDs []int) *scheduler.Topology {
	freeSet := make(map[int]bool)
	for _, id := range freeGPUIDs {
		freeSet[id] = true
	}

	var devices []agent.GPUDevice
	for _, d := range node.Devices {
		if freeSet[d.ID] {
			devices = append(devices, d)
		}
	}

	var links []agent.GPULink
	for _, l := range node.Links {
		if freeSet[l.GPUA] && freeSet[l.GPUB] {
			links = append(links, l)
		}
	}

	return scheduler.BuildTopology(devices, links)
}

func (cs *ClusterScheduler) allocateGPUs(nodeID string, gpuIDs []int, jobID string) {
	if cs.allocated[nodeID] == nil {
		cs.allocated[nodeID] = make(map[int]string)
	}
	for _, id := range gpuIDs {
		cs.allocated[nodeID][id] = jobID
	}
}

func (cs *ClusterScheduler) freeGPUs(nodeID string, gpuIDs []int) {
	if cs.allocated[nodeID] == nil {
		return
	}
	for _, id := range gpuIDs {
		delete(cs.allocated[nodeID], id)
	}
}

// --- Accessors for CLI/Desktop ---

func (cs *ClusterScheduler) RunningJobs() map[string]*ClusterJob {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	result := make(map[string]*ClusterJob, len(cs.running))
	for k, v := range cs.running {
		result[k] = v
	}
	return result
}

func (cs *ClusterScheduler) QueuedJobs() []*scheduler.Job {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	jobs := make([]*scheduler.Job, cs.queue.Len())
	for i, j := range cs.queue {
		jobs[i] = j
	}
	return jobs
}

func (cs *ClusterScheduler) CompletedJobs() []*ClusterJob {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	result := make([]*ClusterJob, len(cs.completed))
	copy(result, cs.completed)
	return result
}

func (cs *ClusterScheduler) QueueLen() int {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.queue.Len()
}

// RecoverRemoteJobs restores jobs from a remote agent into the scheduler.
// Called on reconnect to pick up jobs that survived SSH disconnect.
func (cs *ClusterScheduler) RecoverRemoteJobs(nodeID string, remoteJobs []agentserver.JobStatusResponse) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	for _, rj := range remoteJobs {
		// Skip if already tracked
		if _, exists := cs.running[rj.ID]; exists {
			continue
		}
		// Check completed too
		alreadyCompleted := false
		for _, cj := range cs.completed {
			if cj.ID == rj.ID {
				alreadyCompleted = true
				break
			}
		}
		if alreadyCompleted {
			continue
		}

		startedAt, _ := time.Parse(time.RFC3339, rj.StartedAt)
		cj := &ClusterJob{
			Job: scheduler.Job{
				ID:        rj.ID,
				Command:   rj.Command,
				GPUIDs:    rj.GPUIDs,
				NumGPUs:   len(rj.GPUIDs),
				Status:    scheduler.Running,
				StartedAt: startedAt,
			},
			NodeID: nodeID,
		}

		switch rj.Status {
		case "running":
			cs.running[rj.ID] = cj
			cs.allocateGPUs(nodeID, rj.GPUIDs, rj.ID)
			fmt.Printf("Recovered running job %s from node %s\n", rj.ID, nodeID)
		case "done":
			cj.Status = scheduler.Done
			cs.completed = append(cs.completed, cj)
			fmt.Printf("Recovered completed job %s from node %s\n", rj.ID, nodeID)
		case "failed":
			cj.Status = scheduler.Failed
			cs.completed = append(cs.completed, cj)
			fmt.Printf("Recovered failed job %s from node %s\n", rj.ID, nodeID)
		}
	}
}

func (cs *ClusterScheduler) Stop() {
	close(cs.stopCh)
}
