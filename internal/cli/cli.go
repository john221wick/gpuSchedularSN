package cli

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/john221wick/gpuSchedularSN/internal/agent"
	"github.com/john221wick/gpuSchedularSN/internal/runner"
	"github.com/john221wick/gpuSchedularSN/internal/scheduler"
	"github.com/john221wick/gpuSchedularSN/internal/state"
)

const Version = "0.1.0"

func Run(args []string) {
	if len(args) == 0 {
		printUsage()
		return
	}

	switch args[0] {
	case "devices":
		cmdDevices()
	case "topo":
		cmdTopo()
	case "run":
		cmdRun(args[1:])
	case "status":
		cmdStatus()
	case "kill":
		cmdKill(args[1:])
	case "version":
		cmdVersion()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: gpusched <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  devices   List detected GPUs")
	fmt.Println("  topo      Show GPU topology")
	fmt.Println("  run       Run a command on allocated GPUs")
	fmt.Println("  status    Show running/queued jobs")
	fmt.Println("  kill      Kill a running job")
	fmt.Println("  version   Print version")
}

func cmdDevices() {
	n, err := agent.Init()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	devices := agent.GetDevices()
	if n == 0 {
		fmt.Println("No GPU detected")
		return
	}

	fmt.Printf("Detected %d GPU(s):\n", n)
	for _, d := range devices {
		fmt.Printf("  [%d] %s  %dMB  vendor:%s\n", d.ID, d.Name, d.VRAMTotalMB, d.Vendor)
	}
	agent.Shutdown()
}

func cmdTopo() {
	n, err := agent.Init()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	devices := agent.GetDevices()
	if n == 0 {
		fmt.Println("No GPU detected, no topology")
		agent.Shutdown()
		return
	}

	if n == 1 {
		fmt.Printf("1 GPU (%s), no topology\n", devices[0].Name)
		agent.Shutdown()
		return
	}

	links := agent.GetTopology()
	topo := scheduler.BuildTopology(devices, links)
	if topo == nil {
		fmt.Println("failed to build topology")
		agent.Shutdown()
		return
	}

	fmt.Printf("Topology (%d GPUs):\n\n", n)

	header := "       "
	for j := 0; j < n; j++ {
		header += fmt.Sprintf("  GPU%-2d", j)
	}
	fmt.Println(header)

	for i := 0; i < n; i++ {
		row := fmt.Sprintf("  GPU%-2d ", i)
		for j := 0; j < n; j++ {
			if i == j {
				row += "   ---"
			} else if topo.Bandwidth[i][j] > 0 {
				row += fmt.Sprintf(" %5.0f", topo.Bandwidth[i][j])
			} else {
				row += "     -"
			}
		}
		fmt.Println(row)
	}

	fmt.Printf("\nLinks:\n")
	for _, l := range links {
		fmt.Printf("  GPU%d <-> GPU%d  %s  %.0f GB/s\n", l.GPUA, l.GPUB, l.Type, l.BandwidthGBps)
	}

	agent.Shutdown()
}

func cmdRun(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	gpus := fs.Int("gpus", 1, "number of GPUs to allocate")
	vram := fs.String("vram", "0", "minimum VRAM per GPU (e.g. 40g, 512m)")
	priority := fs.Int("priority", 10, "job priority (lower = higher priority)")

	sepIdx := -1
	for i, a := range args {
		if a == "--" {
			sepIdx = i
			break
		}
	}

	var flagArgs []string
	var cmdArgs []string
	if sepIdx >= 0 {
		flagArgs = args[:sepIdx]
		cmdArgs = args[sepIdx+1:]
	} else {
		flagArgs = args
	}

	fs.Parse(flagArgs)

	if len(cmdArgs) == 0 {
		fmt.Fprintf(os.Stderr, "error: no command specified\n")
		fmt.Fprintf(os.Stderr, "usage: gpusched run --gpus N -- command [args...]\n")
		os.Exit(1)
	}

	minVRAMMB := parseVRAM(*vram)

	n, err := agent.Init()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	devices := agent.GetDevices()
	if n == 0 {
		fmt.Fprintf(os.Stderr, "error: no GPU detected\n")
		agent.Shutdown()
		os.Exit(1)
	}

	links := agent.GetTopology()
	topo := scheduler.BuildTopology(devices, links)

	result := scheduler.Score(topo, *gpus, minVRAMMB)
	if result == nil {
		fmt.Fprintf(os.Stderr, "error: cannot allocate %d GPU(s) with %dMB VRAM\n", *gpus, minVRAMMB)
		agent.Shutdown()
		os.Exit(1)
	}

	vendor := devices[0].Vendor
	fmt.Printf("Allocated GPUs: %v (score: %.0f GB/s)\n", result.GPUIDs, result.Score)
	fmt.Printf("Running: %s\n", strings.Join(cmdArgs, " "))

	st := state.NewState(devices, links)
	job := &scheduler.Job{
		ID:          fmt.Sprintf("job-%d", time.Now().UnixNano()),
		Command:     cmdArgs[0],
		NumGPUs:     *gpus,
		MinVRAMMB:   minVRAMMB,
		Priority:    *priority,
		Status:      scheduler.Running,
		SubmittedAt: time.Now(),
		StartedAt:   time.Now(),
		GPUIDs:      result.GPUIDs,
	}
	st.MarkRunning(job)

	done := make(chan error, 1)
	err = runner.Launch(cmdArgs[0], cmdArgs[1:], result.GPUIDs, vendor, func(e error) {
		done <- e
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		agent.Shutdown()
		os.Exit(1)
	}

	e := <-done
	if e != nil {
		fmt.Fprintf(os.Stderr, "process exited with error: %v\n", e)
		st.MarkFailed(job.ID)
		agent.Shutdown()
		os.Exit(1)
	}

	fmt.Println("Done")
	st.MarkDone(job.ID)
	agent.Shutdown()
}

func cmdStatus() {
	fmt.Println("No jobs running (scheduler loop not started yet)")
}

func cmdKill(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: gpusched kill <jobID>\n")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "kill not implemented yet (requires scheduler loop)\n")
}

func cmdVersion() {
	fmt.Printf("gpusched %s\n", Version)
}

func parseVRAM(s string) uint64 {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "0" || s == "" {
		return 0
	}

	if strings.HasSuffix(s, "gb") {
		s = strings.TrimSuffix(s, "gb")
		n, _ := strconv.ParseUint(s, 10, 64)
		return n * 1000
	}
	if strings.HasSuffix(s, "g") {
		s = strings.TrimSuffix(s, "g")
		n, _ := strconv.ParseUint(s, 10, 64)
		return n * 1000
	}
	if strings.HasSuffix(s, "mb") {
		s = strings.TrimSuffix(s, "mb")
		n, _ := strconv.ParseUint(s, 10, 64)
		return n
	}
	if strings.HasSuffix(s, "m") {
		s = strings.TrimSuffix(s, "m")
		n, _ := strconv.ParseUint(s, 10, 64)
		return n
	}

	n, _ := strconv.ParseUint(s, 10, 64)
	return n
}
