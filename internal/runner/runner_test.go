package runner

import (
	"os"
	"testing"
	"github.com/john221wick/gpuSchedularSN/internal/agent"
)

func TestBuildEnvNVIDIA(t *testing.T) {
	env := BuildEnv([]int{0, 1}, agent.NVIDIA)
	if len(env) != 1 {
		t.Fatalf("expected 1 env var, got %d", len(env))
	}
	if env[0] != "CUDA_VISIBLE_DEVICES=0,1" {
		t.Errorf("got %s, want CUDA_VISIBLE_DEVICES=0,1", env[0])
	}
}

func TestBuildEnvAMD(t *testing.T) {
	env := BuildEnv([]int{2}, agent.AMD)
	if env[0] != "HIP_VISIBLE_DEVICES=2" {
		t.Errorf("got %s, want HIP_VISIBLE_DEVICES=2", env[0])
	}
}

func TestBuildEnvIntel(t *testing.T) {
	env := BuildEnv([]int{0, 1, 2}, agent.Intel)
	if env[0] != "GPU_DEVICE_ORDINAL=0,1,2" {
		t.Errorf("got %s, want GPU_DEVICE_ORDINAL=0,1,2", env[0])
	}
}

func TestBuildEnvApple(t *testing.T) {
	env := BuildEnv([]int{0}, agent.Apple)
	if env != nil {
		t.Errorf("expected nil for Apple, got %v", env)
	}
}

func TestBuildEnvEmpty(t *testing.T) {
	env := BuildEnv(nil, agent.NVIDIA)
	if env != nil {
		t.Errorf("expected nil for empty GPU IDs, got %v", env)
	}
}

func TestLaunchEcho(t *testing.T) {
	done := make(chan error, 1)
	err := Launch("echo", []string{"hello"}, nil, agent.Apple, func(e error) {
		done <- e
	})
	if err != nil {
		t.Fatalf("Launch returned error: %v", err)
	}
	e := <-done
	if e != nil {
		t.Errorf("echo hello failed: %v", e)
	}
}

func TestLaunchFalse(t *testing.T) {
	done := make(chan error, 1)
	err := Launch("false", nil, nil, agent.Apple, func(e error) {
		done <- e
	})
	if err != nil {
		t.Fatalf("Launch returned error: %v", err)
	}
	e := <-done
	if e == nil {
		t.Error("expected error from false, got nil")
	}
}

func TestLaunchSetsCUDA(t *testing.T) {
	done := make(chan error, 1)
	err := Launch("env", nil, []int{0, 1}, agent.NVIDIA, func(e error) {
		done <- e
	})
	if err != nil {
		t.Fatalf("Launch returned error: %v", err)
	}
	<-done
}

func TestLaunchSetsEnvForProcess(t *testing.T) {
	os.Setenv("CUDA_VISIBLE_DEVICES", "")
	done := make(chan error, 1)
	err := Launch("sh", []string{"-c", "echo $CUDA_VISIBLE_DEVICES"}, []int{3, 7}, agent.NVIDIA, func(e error) {
		done <- e
	})
	if err != nil {
		t.Fatalf("Launch returned error: %v", err)
	}
	<-done
}
