package state

import (
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/john221wick/gpuSchedularSN/internal/agent"
	"github.com/john221wick/gpuSchedularSN/internal/scheduler"
)

func mockDevices() []agent.GPUDevice {
	return []agent.GPUDevice{
		{ID: 0, Vendor: agent.NVIDIA, Name: "A100", VRAMTotalMB: 80000, VRAMUsedMB: 0},
		{ID: 1, Vendor: agent.NVIDIA, Name: "A100", VRAMTotalMB: 80000, VRAMUsedMB: 0},
		{ID: 2, Vendor: agent.NVIDIA, Name: "A100", VRAMTotalMB: 80000, VRAMUsedMB: 0},
		{ID: 3, Vendor: agent.NVIDIA, Name: "A100", VRAMTotalMB: 80000, VRAMUsedMB: 0},
	}
}

func mockLinks() []agent.GPULink {
	return []agent.GPULink{
		{GPUA: 0, GPUB: 1, Type: agent.NVLink, BandwidthGBps: 600},
		{GPUA: 2, GPUB: 3, Type: agent.NVLink, BandwidthGBps: 600},
	}
}

func TestNewState(t *testing.T) {
	s := NewState(mockDevices(), mockLinks())
	if s == nil {
		t.Fatal("NewState returned nil")
	}
	if s.Topology() == nil {
		t.Fatal("topology is nil")
	}
}

func TestSubmitAndPopJob(t *testing.T) {
	s := NewState(mockDevices(), mockLinks())

	job := &scheduler.Job{ID: "j1", Command: "echo hello", NumGPUs: 2, Priority: 1, SubmittedAt: time.Now()}
	s.SubmitJob(job)

	if s.QueueLen() != 1 {
		t.Fatalf("queue length = %d, want 1", s.QueueLen())
	}

	popped := s.PopJob()
	if popped == nil {
		t.Fatal("PopJob returned nil")
	}
	if popped.ID != "j1" {
		t.Errorf("popped job ID = %s, want j1", popped.ID)
	}
}

func TestAllocateAndFreeGPUs(t *testing.T) {
	s := NewState(mockDevices(), mockLinks())

	s.AllocateGPUs([]int{0, 1}, "j1")

	if s.IsGPUFree(0) {
		t.Error("GPU 0 should be allocated")
	}
	if s.IsGPUFree(1) {
		t.Error("GPU 1 should be allocated")
	}
	if !s.IsGPUFree(2) {
		t.Error("GPU 2 should be free")
	}

	free := s.GetFreeDevices()
	if len(free) != 2 {
		t.Errorf("free devices = %d, want 2", len(free))
	}

	s.FreeGPUs([]int{0, 1})

	if !s.IsGPUFree(0) {
		t.Error("GPU 0 should be free after FreeGPUs")
	}
	if !s.IsGPUFree(1) {
		t.Error("GPU 1 should be free after FreeGPUs")
	}

	free = s.GetFreeDevices()
	if len(free) != 4 {
		t.Errorf("free devices = %d, want 4", len(free))
	}
}

func TestMarkRunningDone(t *testing.T) {
	s := NewState(mockDevices(), mockLinks())

	job := &scheduler.Job{ID: "j1", Command: "echo hello", NumGPUs: 2, Priority: 1, SubmittedAt: time.Now()}
	s.SubmitJob(job)
	popped := s.PopJob()
	s.MarkRunning(popped)

	running := s.RunningJobs()
	if len(running) != 1 {
		t.Fatalf("running jobs = %d, want 1", len(running))
	}
	if running["j1"].Status != scheduler.Running {
		t.Error("job should be Running")
	}

	s.MarkDone("j1")

	running = s.RunningJobs()
	if len(running) != 0 {
		t.Errorf("running jobs = %d, want 0 after done", len(running))
	}
}

func TestMarkFailed(t *testing.T) {
	s := NewState(mockDevices(), mockLinks())

	job := &scheduler.Job{ID: "j1", Command: "fail", NumGPUs: 1, Priority: 1, SubmittedAt: time.Now()}
	s.SubmitJob(job)
	popped := s.PopJob()
	s.MarkRunning(popped)
	s.MarkFailed("j1")

	running := s.RunningJobs()
	if len(running) != 0 {
		t.Errorf("running jobs = %d, want 0 after fail", len(running))
	}
}

func TestKillJobKillsProcessAndFreesGPUs(t *testing.T) {
	s := NewState(mockDevices(), mockLinks())
	job := &scheduler.Job{
		ID:          "kill-me",
		Command:     "sleep 30",
		NumGPUs:     1,
		Priority:    1,
		Status:      scheduler.Running,
		SubmittedAt: time.Now(),
		StartedAt:   time.Now(),
		GPUIDs:      []int{0},
	}

	s.AllocateGPUs(job.GPUIDs, job.ID)
	s.MarkRunning(job)

	cmd := exec.Command("sh", "-c", "sleep 30")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start test process: %v", err)
	}
	s.StoreProc(job.ID, cmd, job)

	if err := s.KillJob(job.ID); err != nil {
		t.Fatalf("KillJob returned error: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("process was still running after KillJob")
	}

	if running := s.RunningJobs(); len(running) != 0 {
		t.Errorf("running jobs = %d, want 0 after kill", len(running))
	}
	if !s.IsGPUFree(0) {
		t.Error("GPU 0 should be free after KillJob")
	}

	completed := s.CompletedJobs()
	if len(completed) != 1 {
		t.Fatalf("completed jobs = %d, want 1", len(completed))
	}
	if completed[0].Status != scheduler.Failed {
		t.Errorf("completed status = %s, want Failed", completed[0].Status)
	}
}
