package desktop

import (
	"strings"
	"testing"

	"github.com/john221wick/gpuSchedularSN/internal/state"
)

func TestNormalizeSubmitCommandExpandsRelativePythonScript(t *testing.T) {
	command, err := normalizeSubmitCommand("python3 test/train.py --epochs 2", "")
	if err != nil {
		t.Fatalf("normalizeSubmitCommand returned error: %v", err)
	}

	if !strings.HasPrefix(command, "python3 '") {
		t.Fatalf("expanded command = %q, want python3 quoted path", command)
	}
	if !strings.Contains(command, "/test/train.py' --epochs 2") {
		t.Fatalf("expanded command = %q, want test/train.py with suffix", command)
	}
}

func TestNormalizeSubmitCommandExpandsBareRelativePythonScript(t *testing.T) {
	command, err := normalizeSubmitCommand("test/train.py --epochs 2", "")
	if err != nil {
		t.Fatalf("normalizeSubmitCommand returned error: %v", err)
	}
	if !strings.HasPrefix(command, "python3 '") {
		t.Fatalf("expanded command = %q, want python3 quoted path", command)
	}
	if !strings.Contains(command, "/test/train.py' --epochs 2") {
		t.Fatalf("expanded command = %q, want test/train.py with suffix", command)
	}
}

func TestNormalizeSubmitCommandPassThrough(t *testing.T) {
	command, err := normalizeSubmitCommand("echo hello", "")
	if err != nil {
		t.Fatalf("normalizeSubmitCommand returned error: %v", err)
	}
	if command != "echo hello" {
		t.Fatalf("command = %q, want original command", command)
	}
}

func TestNormalizeSubmitCommandUsesPathVariableDirectory(t *testing.T) {
	basePath, err := resolveRelativeRepoPath("test")
	if err != nil {
		t.Fatalf("resolveRelativeRepoPath returned error: %v", err)
	}

	command, err := normalizeSubmitCommand("python3 train.py --epochs 2", basePath)
	if err != nil {
		t.Fatalf("normalizeSubmitCommand returned error: %v", err)
	}

	if !strings.HasPrefix(command, "python3 '") {
		t.Fatalf("expanded command = %q, want python3 quoted path", command)
	}
	if !strings.Contains(command, "/test/train.py' --epochs 2") {
		t.Fatalf("expanded command = %q, want path variable directory with suffix", command)
	}
}

func TestSubmitJobKeepsDisplayCommandShort(t *testing.T) {
	app := &App{state: state.NewState(nil, nil)}
	jobID, err := app.SubmitJob(SubmitRequest{
		Command: "python3 test/train.py --epochs 2",
		NumGPUs: 1,
	})
	if err != nil {
		t.Fatalf("SubmitJob returned error: %v", err)
	}

	queued := app.state.QueuedJobs()
	if len(queued) != 1 {
		t.Fatalf("queued jobs = %d, want 1", len(queued))
	}
	if queued[0].ID != jobID {
		t.Fatalf("queued job ID = %q, want %q", queued[0].ID, jobID)
	}
	if queued[0].Command != "python3 test/train.py --epochs 2" {
		t.Fatalf("display command = %q, want original short command", queued[0].Command)
	}
	if !strings.Contains(queued[0].ExecCommand, "/test/train.py' --epochs 2") {
		t.Fatalf("exec command = %q, want resolved script path", queued[0].ExecCommand)
	}
}

func TestSubmitJobUsesPathVariableWithoutChangingDisplayCommand(t *testing.T) {
	basePath, err := resolveRelativeRepoPath("test")
	if err != nil {
		t.Fatalf("resolveRelativeRepoPath returned error: %v", err)
	}

	app := &App{state: state.NewState(nil, nil)}
	_, err = app.SubmitJob(SubmitRequest{
		Command:      "python3 train.py",
		PathVariable: basePath,
		NumGPUs:      1,
	})
	if err != nil {
		t.Fatalf("SubmitJob returned error: %v", err)
	}

	queued := app.state.QueuedJobs()
	if len(queued) != 1 {
		t.Fatalf("queued jobs = %d, want 1", len(queued))
	}
	if queued[0].Command != "python3 train.py" {
		t.Fatalf("display command = %q, want original short command", queued[0].Command)
	}
	if !strings.Contains(queued[0].ExecCommand, "/test/train.py'") {
		t.Fatalf("exec command = %q, want path variable resolved script path", queued[0].ExecCommand)
	}
}
