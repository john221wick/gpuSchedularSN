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
	job  *scheduler.Job
	cmd  *exec.Cmd
	done <-chan error
}

type State struct {
	mu           sync.Mutex
	devices      []agent.GPUDevice
	links        []agent.GPULink
	topology     *scheduler.Topology
	queue        scheduler.JobQueue
	running      map[string]*scheduler.Job
	runningProcs map[string]*runningProc
	completed    []*scheduler.Job
	allocated    map[int]string
	stopCh       chan struct{}
	loopRunning  bool
	stopping     bool
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

func (s *State) QueuedJobs() []*scheduler.Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	jobs := make([]*scheduler.Job, s.queue.Len())
	for i := range s.queue {
		jobs[i] = s.queue[i]
	}
	return jobs
}

func (s *State) RemoveQueuedJob(jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job := s.queue.RemoveByID(jobID)
	if job == nil {
		return fmt.Errorf("job %s not found in queue", jobID)
	}
	return nil
}

func (s *State) UpdateQueuedPriority(jobID string, newPriority int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job := s.queue.RemoveByID(jobID)
	if job == nil {
		return fmt.Errorf("job %s not found in queue", jobID)
	}
	job.Priority = newPriority
	s.queue.PushJob(job)
	return nil
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
	s.finishRunningJobLocked(jobID, scheduler.Done)
}

func (s *State) MarkFailed(jobID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.finishRunningJobLocked(jobID, scheduler.Failed)
}

func (s *State) finishRunningJobLocked(jobID string, status scheduler.JobStatus) {
	if job, ok := s.running[jobID]; ok {
		job.Status = status
		for _, id := range job.GPUIDs {
			delete(s.allocated, id)
		}
		s.completed = append(s.completed, job)
		delete(s.running, jobID)
		delete(s.runningProcs, jobID)
	}
}

func (s *State) CompletedJobs() []*scheduler.Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]*scheduler.Job, len(s.completed))
	copy(result, s.completed)
	return result
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

func (s *State) StartSchedulerLoop() {
	s.mu.Lock()
	if s.loopRunning {
		s.mu.Unlock()
		return
	}
	s.loopRunning = true
	s.stopping = false
	s.stopCh = make(chan struct{})
	s.mu.Unlock()

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-s.stopCh:
				return
			default:
			}

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

	if s.stopping {
		return
	}

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

	// Check all scored GPUs are actually free
	for _, id := range result.GPUIDs {
		if _, taken := s.allocated[id]; taken {
			return
		}
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
	jobID := job.ID
	gpuIDs := result.GPUIDs

	command := job.Command
	if job.ExecCommand != "" {
		command = job.ExecCommand
	}

	doneCh := make(chan error, 1)
	cmd, err := runner.LaunchAsync(command, nil, gpuIDs, vendor, func(e error) {
		doneCh <- e
		if s.IsStopping() {
			return
		}
		if e != nil {
			s.MarkFailed(jobID)
			return
		}
		s.MarkDone(jobID)
	})
	if err != nil {
		s.finishRunningJobLocked(jobID, scheduler.Failed)
		return
	}

	if cmd.Process != nil {
		job.PID = cmd.Process.Pid
	}
	s.runningProcs[jobID] = &runningProc{job: job, cmd: cmd, done: doneCh}
}

func (s *State) Stop() {
	s.mu.Lock()
	if s.stopping {
		s.mu.Unlock()
		return
	}
	s.stopping = true
	if s.loopRunning && s.stopCh != nil {
		close(s.stopCh)
	}
	s.loopRunning = false

	procs := make([]*runningProc, 0, len(s.runningProcs))
	for _, proc := range s.runningProcs {
		if proc != nil {
			procs = append(procs, proc)
		}
	}

	for jobID := range s.running {
		s.finishRunningJobLocked(jobID, scheduler.Failed)
	}
	s.mu.Unlock()

	for _, proc := range procs {
		if proc == nil || proc.cmd == nil || proc.cmd.Process == nil {
			continue
		}
		_ = killProcessTree(proc.cmd.Process.Pid)
		waitForProcessDone(proc, 2*time.Second)
	}
}

func (s *State) IsStopping() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopping
}

func (s *State) KillJob(jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, jobExists := s.running[jobID]
	if !jobExists {
		return fmt.Errorf("job %s not found", jobID)
	}

	proc := s.runningProcs[jobID]
	if proc == nil || proc.cmd == nil || proc.cmd.Process == nil {
		return fmt.Errorf("job %s process is not ready", jobID)
	}

	pid := proc.cmd.Process.Pid
	if err := killProcessTree(pid); err != nil {
		return fmt.Errorf("failed to kill job %s: %w", jobID, err)
	}

	s.finishRunningJobLocked(jobID, scheduler.Failed)
	return nil
}

func killProcessTree(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid %d", pid)
	}

	groupErr := syscall.Kill(-pid, syscall.SIGKILL)
	if groupErr == nil {
		return nil
	}

	processErr := syscall.Kill(pid, syscall.SIGKILL)
	if processErr == nil || processErr == syscall.ESRCH {
		return nil
	}

	return fmt.Errorf("process group: %v, process: %w", groupErr, processErr)
}

func waitForProcessDone(proc *runningProc, timeout time.Duration) {
	if proc.done == nil {
		return
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-proc.done:
	case <-timer.C:
	}
}
