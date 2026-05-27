package runner

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/john221wick/gpuSchedularSN/internal/agent"
)

func BuildEnv(gpuIDs []int, vendor agent.Vendor) []string {
	if vendor == agent.Apple {
		return nil
	}

	if len(gpuIDs) == 0 {
		return nil
	}

	ids := make([]string, len(gpuIDs))
	for i, id := range gpuIDs {
		ids[i] = strconv.Itoa(id)
	}
	csv := strings.Join(ids, ",")

	var env []string
	switch vendor {
	case agent.NVIDIA:
		env = append(env, "CUDA_VISIBLE_DEVICES="+csv)
	case agent.AMD:
		env = append(env, "HIP_VISIBLE_DEVICES="+csv)
	case agent.Intel:
		env = append(env, "GPU_DEVICE_ORDINAL="+csv)
	default:
		env = append(env, "CUDA_VISIBLE_DEVICES="+csv)
	}

	return env
}

func Launch(command string, args []string, gpuIDs []int, vendor agent.Vendor, onDone func(error)) error {
	_, err := LaunchAsync(command, args, gpuIDs, vendor, onDone)
	return err
}

func LaunchAsync(command string, args []string, gpuIDs []int, vendor agent.Vendor, onDone func(error)) (*exec.Cmd, error) {
	env := BuildEnv(gpuIDs, vendor)

	var cmd *exec.Cmd
	if len(args) == 0 {
		// Command is a full shell string like "python3 main.py --epochs 10"
		cmd = exec.Command("sh", "-c", command)
	} else {
		cmd = exec.Command(command, args...)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), env...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start %s: %w", command, err)
	}

	go func() {
		err := cmd.Wait()
		if onDone != nil {
			onDone(err)
		}
	}()

	return cmd, nil
}
