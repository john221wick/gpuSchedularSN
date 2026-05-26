package state

import (
	"sync"
	"github.com/john221wick/gpuSchedularSN/internal/agent"
	"github.com/john221wick/gpuSchedularSN/internal/scheduler"
)

type State struct {
	mu        sync.Mutex
	devices   []agent.GPUDevice
	links     []agent.GPULink
	topology  *scheduler.Topology
	queue     scheduler.JobQueue
	running   map[string]*scheduler.Job
	allocated map[int]string
}

func NewState(devices []agent.GPUDevice, links []agent.GPULink) *State {
	topo := scheduler.BuildTopology(devices, links)
	q := &scheduler.JobQueue{}
	return &State{
		devices:   devices,
		links:     links,
		topology:  topo,
		queue:     *q,
		running:   make(map[string]*scheduler.Job),
		allocated: make(map[int]string),
	}
}

func (s *State) SubmitJob(job *scheduler.Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queue.PushJob(job)
}

func (s *State) AllocateGPUs(gpuIDs []int, jobID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range gpuIDs {
		s.allocated[id] = jobID
	}
}

func (s *State) FreeGPUs(gpuIDs []int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range gpuIDs {
		delete(s.allocated, id)
	}
}

func (s *State) IsGPUFree(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, taken := s.allocated[id]
	return !taken
}

func (s *State) GetFreeDevices() []agent.GPUDevice {
	s.mu.Lock()
	defer s.mu.Unlock()
	var free []agent.GPUDevice
	for _, d := range s.devices {
		if _, taken := s.allocated[d.ID]; !taken {
			free = append(free, d)
		}
	}
	return free
}

func (s *State) Topology() *scheduler.Topology {
	return s.topology
}

func (s *State) QueueLen() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.queue.Len()
}

func (s *State) PopJob() *scheduler.Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.queue.PopJob()
}

func (s *State) MarkRunning(job *scheduler.Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job.Status = scheduler.Running
	s.running[job.ID] = job
}

func (s *State) MarkDone(jobID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job, ok := s.running[jobID]; ok {
		job.Status = scheduler.Done
		delete(s.running, jobID)
	}
}

func (s *State) MarkFailed(jobID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job, ok := s.running[jobID]; ok {
		job.Status = scheduler.Failed
		delete(s.running, jobID)
	}
}

func (s *State) RunningJobs() map[string]*scheduler.Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make(map[string]*scheduler.Job)
	for k, v := range s.running {
		result[k] = v
	}
	return result
}
