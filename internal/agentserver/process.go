package agentserver

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/john221wick/gpuSchedularSN/internal/agent"
	"github.com/john221wick/gpuSchedularSN/internal/runner"
)

type managedProcess struct {
	spec      JobSpec
	cmd       *exec.Cmd
	logFile   *os.File
	status    string // "running", "done", "failed"
	pid       int
	startedAt time.Time
	exitCode  int
	mu        sync.Mutex
}

type ProcessManager struct {
	mu        sync.RWMutex
	processes map[string]*managedProcess
	logDir    string
	vendor    agent.Vendor
	// SaveFn is called after job state changes (start, complete, kill).
	// Set by AgentServer to trigger immediate state persistence.
	SaveFn func()
}

func NewProcessManager(logDir string, vendor agent.Vendor) *ProcessManager {
	return &ProcessManager{
		processes: make(map[string]*managedProcess),
		logDir:    logDir,
		vendor:    vendor,
	}
}

func (pm *ProcessManager) Start(spec JobSpec) error {
	pm.mu.Lock()
	if _, exists := pm.processes[spec.ID]; exists {
		pm.mu.Unlock()
		return fmt.Errorf("job %s already exists", spec.ID)
	}
	pm.mu.Unlock()

	logPath := fmt.Sprintf("%s/%s.log", pm.logDir, spec.ID)
	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("create log file: %w", err)
	}

	env := runner.BuildEnv(spec.GPUIDs, pm.vendor)

	cmd := exec.Command("sh", "-c", spec.Command)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = append(os.Environ(), env...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if spec.WorkDir != "" {
		cmd.Dir = spec.WorkDir
	}

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("start process: %w", err)
	}

	proc := &managedProcess{
		spec:      spec,
		cmd:       cmd,
		logFile:   logFile,
		status:    "running",
		pid:       cmd.Process.Pid,
		startedAt: time.Now(),
	}

	pm.mu.Lock()
	pm.processes[spec.ID] = proc
	pm.mu.Unlock()

	// Persist immediately after job start
	if pm.SaveFn != nil {
		pm.SaveFn()
	}

	go func() {
		waitErr := cmd.Wait()
		proc.mu.Lock()
		if proc.status == "running" {
			if waitErr == nil {
				proc.status = "done"
				proc.exitCode = 0
			} else {
				proc.status = "failed"
				if exitErr, ok := waitErr.(*exec.ExitError); ok {
					proc.exitCode = exitErr.ExitCode()
				} else {
					proc.exitCode = -1
				}
			}
		}
		proc.mu.Unlock()
		logFile.Close()

		// Persist after job completes
		if pm.SaveFn != nil {
			pm.SaveFn()
		}
	}()

	return nil
}

func (pm *ProcessManager) Kill(jobID string) error {
	pm.mu.RLock()
	proc, exists := pm.processes[jobID]
	pm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("job %s not found", jobID)
	}

	proc.mu.Lock()
	defer proc.mu.Unlock()

	if proc.status != "running" {
		return fmt.Errorf("job %s is not running (status: %s)", jobID, proc.status)
	}

	pid := proc.pid
	if pid <= 0 {
		return fmt.Errorf("job %s has invalid pid", jobID)
	}

	// SIGTERM first
	_ = syscall.Kill(-pid, syscall.SIGTERM)

	// Wait up to 5s for graceful exit
	done := make(chan struct{})
	go func() {
		for i := 0; i < 50; i++ {
			time.Sleep(100 * time.Millisecond)
			if err := syscall.Kill(pid, 0); err != nil {
				close(done)
				return
			}
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}

	// Check if still alive, SIGKILL if needed
	if err := syscall.Kill(pid, 0); err == nil {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	}

	proc.status = "failed"
	proc.exitCode = -1
	return nil
}

func (pm *ProcessManager) Status() []JobStatusResponse {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make([]JobStatusResponse, 0, len(pm.processes))
	for _, proc := range pm.processes {
		proc.mu.Lock()
		result = append(result, JobStatusResponse{
			ID:        proc.spec.ID,
			Command:   proc.spec.Command,
			GPUIDs:    proc.spec.GPUIDs,
			PID:       proc.pid,
			Status:    proc.status,
			StartedAt: proc.startedAt.Format(time.RFC3339),
			ExitCode:  proc.exitCode,
		})
		proc.mu.Unlock()
	}
	return result
}

func (pm *ProcessManager) ReadLog(jobID string, offset int64) (LogChunk, error) {
	// Check if job exists (even if log file not created yet)
	pm.mu.RLock()
	_, jobExists := pm.processes[jobID]
	pm.mu.RUnlock()

	logPath := fmt.Sprintf("%s/%s.log", pm.logDir, jobID)
	f, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) && jobExists {
			// Job exists but log file not created yet — return empty
			return LogChunk{Data: "", Offset: 0, EOF: false}, nil
		}
		return LogChunk{}, fmt.Errorf("open log: %w", err)
	}
	defer f.Close()

	if offset > 0 {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return LogChunk{}, fmt.Errorf("seek: %w", err)
		}
	}

	data, err := io.ReadAll(f)
	if err != nil {
		return LogChunk{}, fmt.Errorf("read: %w", err)
	}

	newOffset := offset + int64(len(data))

	// Check if job is still running
	eof := true
	pm.mu.RLock()
	if proc, exists := pm.processes[jobID]; exists {
		proc.mu.Lock()
		if proc.status == "running" {
			eof = false
		}
		proc.mu.Unlock()
	}
	pm.mu.RUnlock()

	return LogChunk{
		Data:   string(data),
		Offset: newOffset,
		EOF:    eof && len(data) == 0,
	}, nil
}

func (pm *ProcessManager) ReconcilePIDs() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for _, proc := range pm.processes {
		proc.mu.Lock()
		if proc.status == "running" && proc.pid > 0 {
			if err := syscall.Kill(proc.pid, 0); err != nil {
				proc.status = "failed"
				proc.exitCode = -1
			}
		}
		proc.mu.Unlock()
	}
}

// RestoreFromState loads persisted jobs into the process manager for PID reconciliation.
// For still-running processes, starts a background goroutine to track completion.
func (pm *ProcessManager) RestoreFromState(jobs []PersistedJob) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for _, pj := range jobs {
		startedAt, _ := time.Parse(time.RFC3339, pj.StartedAt)
		proc := &managedProcess{
			spec: JobSpec{
				ID:      pj.ID,
				Command: pj.Command,
				GPUIDs:  pj.GPUIDs,
				WorkDir: pj.WorkDir,
			},
			status:    pj.Status,
			pid:       pj.PID,
			startedAt: startedAt,
		}
		pm.processes[pj.ID] = proc

		// For running processes, monitor PID in background
		if pj.Status == "running" && pj.PID > 0 {
			go pm.monitorRestoredProcess(proc)
		}
	}
}

// monitorRestoredProcess polls a restored process PID until it exits.
func (pm *ProcessManager) monitorRestoredProcess(proc *managedProcess) {
	for {
		time.Sleep(2 * time.Second)
		proc.mu.Lock()
		if proc.status != "running" {
			proc.mu.Unlock()
			return
		}
		pid := proc.pid
		proc.mu.Unlock()

		if err := syscall.Kill(pid, 0); err != nil {
			// Process gone — check exit via /proc or mark failed
			proc.mu.Lock()
			if proc.status == "running" {
				// Can't get real exit code from restored process, assume failed
				// Check if log ends with success indicators
				proc.status = "done"
				proc.exitCode = 0
			}
			proc.mu.Unlock()
			return
		}
	}
}

// ToPersistedState converts current state for saving.
func (pm *ProcessManager) ToPersistedState() *PersistedState {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var jobs []PersistedJob
	for _, proc := range pm.processes {
		proc.mu.Lock()
		jobs = append(jobs, PersistedJob{
			ID:        proc.spec.ID,
			Command:   proc.spec.Command,
			GPUIDs:    proc.spec.GPUIDs,
			PID:       proc.pid,
			Status:    proc.status,
			StartedAt: proc.startedAt.Format(time.RFC3339),
			WorkDir:   proc.spec.WorkDir,
		})
		proc.mu.Unlock()
	}

	return &PersistedState{Jobs: jobs}
}
