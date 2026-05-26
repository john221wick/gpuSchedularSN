package state

import (
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/john221wick/gpuSchedularSN/internal/agent"
	"github.com/john221wick/gpuSchedularSN/internal/runner"
	"github.com/john221wick/gpuSchedularSN/internal/scheduler"
)

type runningProc struct {
	job *scheduler.Job
	cmd *exec.Cmd
}

type State struct {
	mu           sync.Mutex
	devices      []agent.GPUDevice
	links        []agent.GPULink
	topology     *scheduler.Topology
	queue        scheduler.JobQueue
	running      map[string]*scheduler.Job
	runningProcs map[string]*runningProc
	allocated    map[int]string
	stopCh       chan struct{}
	loopRunning  bool
	doneCh       map[string]chan error
}

func NewState(devices []agent.GPUDevice, links []agent.GPULink) *State {
	topo := scheduler.BuildTopology(devices, links)
	q := &scheduler.JobQueue{}
	return &State{
		devices:      devices,
		links:        links,
		topology:     topo,
		queue:        *q,
		running:      make(map[string]*scheduler.Job),
		runningProcs: make(map[string]*runningProc),
		allocated:    make(map[int]string),
		doneCh:       make(map[string]chan error),
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

func (s *State) Devices() []agent.GPUDevice {
	return s.devices
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
		delete(s.runningProcs, jobID)
	}
}

func (s *State) MarkFailed(jobID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job, ok := s.running[jobID]; ok {
		job.Status = scheduler.Failed
		delete(s.running, jobID)
		delete(s.runningProcs, jobID)
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

func (s *State) StoreProc(jobID string, cmd *exec.Cmd, job *scheduler.Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runningProcs[jobID] = &runningProc{job: job, cmd: cmd}
}

func (s *State) StoreDoneCh(jobID string, ch chan error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.doneCh[jobID] = ch
}

func (s *State) GetDoneCh(jobID string) chan error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.doneCh[jobID]
}

func (s *State) StartSchedulerLoop() {
	s.mu.Lock()
	if s.loopRunning {
		s.mu.Unlock()
		return
	}
	s.loopRunning = true
	s.stopCh = make(chan struct{})
	s.mu.Unlock()

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-s.stopCh:
				return
			case <-ticker.C:
				s.tick()
			}
		}
	}()
}

func (s *State) tick() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.queue.Len() == 0 {
		return
	}

	top := s.queue.PeekJob()
	if top == nil {
		return
	}

	result := scheduler.Score(s.topology, top.NumGPUs, top.MinVRAMMB)
	if result == nil {
		return
	}

	job := s.queue.PopJob()
	job.GPUIDs = result.GPUIDs
	job.Status = scheduler.Running
	job.StartedAt = time.Now()
	s.running[job.ID] = job

	for _, id := range result.GPUIDs {
		s.allocated[id] = job.ID
	}

	vendor := s.devices[0].Vendor
	doneCh := make(chan error, 1)
	s.doneCh[job.ID] = doneCh

	go func() {
		cmd, err := runner.LaunchAsync(job.Command, nil, result.GPUIDs, vendor, func(e error) {
			doneCh <- e
		})
		if err != nil {
			s.mu.Lock()
			s.runningProcs[job.ID] = nil
			s.mu.Unlock()
			doneCh <- err
			return
		}
		s.mu.Lock()
		s.runningProcs[job.ID] = &runningProc{job: job, cmd: cmd}
		s.mu.Unlock()
	}()
}

func (s *State) Stop() {
	s.mu.Lock()
	if !s.loopRunning {
		s.mu.Unlock()
		return
	}
	close(s.stopCh)
	s.loopRunning = false

	for _, proc := range s.runningProcs {
		if proc != nil && proc.cmd != nil && proc.cmd.Process != nil {
			syscall.Kill(-proc.cmd.Process.Pid, syscall.SIGKILL)
		}
	}
	s.mu.Unlock()
}

func (s *State) KillJob(jobID string) error {
	s.mu.Lock()
	proc, ok := s.runningProcs[jobID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("job %s not found", jobID)
	}

	if proc != nil && proc.cmd != nil && proc.cmd.Process != nil {
		syscall.Kill(-proc.cmd.Process.Pid, syscall.SIGKILL)
	}

	gpuIDs := proc.job.GPUIDs
	s.mu.Unlock()

	s.FreeGPUs(gpuIDs)
	s.MarkFailed(jobID)
	return nil
}
