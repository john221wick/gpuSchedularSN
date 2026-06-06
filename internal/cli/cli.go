package cli

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/john221wick/gpuSchedularSN/internal/agent"
	"github.com/john221wick/gpuSchedularSN/internal/cluster"
	"github.com/john221wick/gpuSchedularSN/internal/runner"
	"github.com/john221wick/gpuSchedularSN/internal/scheduler"
	"github.com/john221wick/gpuSchedularSN/internal/state"
)

const Version = "0.1.5"

var GlobalState *state.State
var ClusterMgr *cluster.NodeManager
var ClusterSched *cluster.ClusterScheduler

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
	case "connect":
		cmdConnect(args[1:])
	case "nodes":
		cmdNodes()
	case "logs":
		cmdLogs(args[1:])
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
	fmt.Println("  connect   Connect to a remote agent node")
	fmt.Println("  nodes     List connected nodes")
	fmt.Println("  logs      View job logs")
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

	if GlobalState == nil {
		GlobalState = state.NewState(devices, links)
	}

	topo := scheduler.BuildTopology(devices, links)
	result := scheduler.Score(topo, *gpus, minVRAMMB)
	if result == nil {
		fmt.Fprintf(os.Stderr, "error: cannot allocate %d GPU(s) with %dMB VRAM\n", *gpus, minVRAMMB)
		agent.Shutdown()
		os.Exit(1)
	}

	vendor := devices[0].Vendor
	jobID := fmt.Sprintf("job-%d", time.Now().UnixNano())

	job := &scheduler.Job{
		ID:          jobID,
		Command:     cmdArgs[0],
		NumGPUs:     *gpus,
		MinVRAMMB:   minVRAMMB,
		Priority:    *priority,
		Status:      scheduler.Running,
		SubmittedAt: time.Now(),
		StartedAt:   time.Now(),
		GPUIDs:      result.GPUIDs,
	}

	GlobalState.AllocateGPUs(result.GPUIDs, jobID)
	GlobalState.MarkRunning(job)

	fmt.Printf("Allocated GPUs: %v (score: %.0f GB/s)\n", result.GPUIDs, result.Score)
	fmt.Printf("Running: %s\n", strings.Join(cmdArgs, " "))

	doneCh := make(chan error, 1)

	_, err = runner.LaunchAsync(cmdArgs[0], cmdArgs[1:], result.GPUIDs, vendor, func(e error) {
		doneCh <- e
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		GlobalState.FreeGPUs(result.GPUIDs)
		GlobalState.MarkFailed(jobID)
		agent.Shutdown()
		os.Exit(1)
	}

	e := <-doneCh
	if e != nil {
		fmt.Fprintf(os.Stderr, "process exited with error: %v\n", e)
		GlobalState.FreeGPUs(result.GPUIDs)
		GlobalState.MarkFailed(jobID)
		agent.Shutdown()
		os.Exit(1)
	}

	fmt.Println("Done")
	GlobalState.FreeGPUs(result.GPUIDs)
	GlobalState.MarkDone(jobID)
	agent.Shutdown()
}

func cmdStatus() {
	if GlobalState == nil {
		fmt.Println("No state initialized (run a job first)")
		return
	}

	running := GlobalState.RunningJobs()
	queueLen := GlobalState.QueueLen()

	if len(running) == 0 && queueLen == 0 {
		fmt.Println("No jobs running or queued")
		return
	}

	if len(running) > 0 {
		fmt.Printf("Running (%d):\n", len(running))
		for id, job := range running {
			fmt.Printf("  %s  %s  GPUs:%v\n", id, job.Command, job.GPUIDs)
		}
	}

	if queueLen > 0 {
		fmt.Printf("Queued: %d\n", queueLen)
	}
}

func cmdKill(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: gpusched kill <jobID>\n")
		os.Exit(1)
	}

	if GlobalState == nil {
		fmt.Fprintf(os.Stderr, "error: no state initialized\n")
		os.Exit(1)
	}

	jobID := args[0]
	err := GlobalState.KillJob(jobID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Killed job %s\n", jobID)
}

func cmdVersion() {
	fmt.Printf("gpusched %s\n", Version)
}

// initCluster ensures NodeManager and ClusterScheduler are initialized.
func initCluster() {
	if ClusterMgr == nil {
		ClusterMgr = cluster.NewNodeManager()
		ClusterSched = cluster.NewClusterScheduler(ClusterMgr)
	}
}

func cmdConnect(args []string) {
	fs := flag.NewFlagSet("connect", flag.ExitOnError)
	direct := fs.String("direct", "", "direct TCP address (host:port) — skip SSH, for testing")
	key := fs.String("key", "", "path to SSH private key")
	fs.Parse(args)

	initCluster()

	if *direct != "" {
		// Direct TCP connection — no SSH, for local testing
		url := "http://" + *direct
		client := cluster.NewAgentClient(url)
		nodeID := fmt.Sprintf("direct-%s", *direct)

		node, err := ClusterMgr.AddRemoteNode(nodeID, *direct, client)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Connected to %s: %d GPUs\n", node.Name, len(node.Devices))
		return
	}

	// SSH connection
	remaining := fs.Args()
	if len(remaining) == 0 {
		fmt.Fprintf(os.Stderr, "usage: gpusched connect \"ssh -p PORT user@host\" [--key path]\n")
		fmt.Fprintf(os.Stderr, "       gpusched connect --direct host:port\n")
		os.Exit(1)
	}

	rawSSH := remaining[0]
	config, err := cluster.ParseSSHCommand(rawSSH)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing SSH command: %v\n", err)
		os.Exit(1)
	}
	if *key != "" {
		config.KeyPath = *key
	}

	fmt.Printf("Connecting to %s@%s:%d...\n", config.User, config.Host, config.Port)

	session, err := cluster.Connect(*config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "SSH connection failed: %v\n", err)
		os.Exit(1)
	}

	remoteBase := "~/gpuschedular"
	session.RunCommand(fmt.Sprintf("mkdir -p %s/logs", remoteBase))

	// Detect hostname
	hostnameOut, _ := session.RunCommand("hostname")
	hostname := strings.TrimSpace(hostnameOut)

	fmt.Println("SSH connected. Ensuring rsync installed on remote...")
	session.RunCommand("which rsync > /dev/null 2>&1 || (apt-get update -qq && apt-get install -y -qq rsync 2>/dev/null || yum install -y -q rsync 2>/dev/null || apk add rsync 2>/dev/null || true)")

	fmt.Println("Cross-compiling and deploying agent...")
	if err := session.CrossCompileAndSCP(remoteBase+"/gpusched", false); err != nil {
		fmt.Fprintf(os.Stderr, "Deploy failed: %v\n", err)
		os.Exit(1)
	}

	agentPort := 9712
	if err := session.StartRemoteAgent(agentPort, remoteBase, 0); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start remote agent: %v\n", err)
		os.Exit(1)
	}

	localPort, err := session.ForwardPort(agentPort)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Port forward failed: %v\n", err)
		os.Exit(1)
	}

	client := cluster.NewAgentClient(fmt.Sprintf("http://localhost:%d", localPort))
	nodeID := fmt.Sprintf("ssh-%s-%d", config.Host, config.Port)

	// Use detected hostname if available, otherwise fall back to SSH host
	nodeName := hostname
	if nodeName == "" {
		nodeName = config.Host
	}

	node, err := ClusterMgr.AddRemoteNode(nodeID, nodeName, client)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error adding node: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Connected to %s: %d GPUs\n", node.Name, len(node.Devices))
}

func cmdNodes() {
	if ClusterMgr == nil {
		fmt.Println("No cluster initialized. Use 'connect' first.")
		return
	}

	nodes := ClusterMgr.AllNodes()
	if len(nodes) == 0 {
		fmt.Println("No nodes connected")
		return
	}

	fmt.Printf("Nodes (%d):\n", len(nodes))
	for _, n := range nodes {
		fmt.Printf("  %-20s  %-12s  %d GPUs  %s\n", n.Name, n.ID, len(n.Devices), n.Status.String())
	}
}

func cmdLogs(args []string) {
	fs := flag.NewFlagSet("logs", flag.ExitOnError)
	follow := fs.Bool("follow", false, "follow log output")
	fs.Parse(args)

	remaining := fs.Args()
	if len(remaining) == 0 {
		fmt.Fprintf(os.Stderr, "usage: gpusched logs <jobID> [--follow]\n")
		os.Exit(1)
	}

	jobID := remaining[0]

	if ClusterMgr == nil {
		fmt.Fprintf(os.Stderr, "error: no cluster initialized\n")
		os.Exit(1)
	}

	// Find which node has this job
	var nodeID string
	if ClusterSched != nil {
		for id, cj := range ClusterSched.RunningJobs() {
			if id == jobID {
				nodeID = cj.NodeID
				break
			}
		}
	}

	// If not found in running, try all nodes
	if nodeID == "" {
		for _, n := range ClusterMgr.AllNodes() {
			client, ok := ClusterMgr.GetClient(n.ID)
			if !ok {
				continue
			}
			chunk, err := client.GetLogs(jobID, 0)
			if err == nil {
				nodeID = n.ID
				fmt.Print(chunk.Data)
				if !*follow || chunk.EOF {
					return
				}
				// Continue following from this offset
				followLogs(n.ID, jobID, chunk.Offset)
				return
			}
		}
		fmt.Fprintf(os.Stderr, "error: job %s not found on any node\n", jobID)
		os.Exit(1)
	}

	client, ok := ClusterMgr.GetClient(nodeID)
	if !ok {
		fmt.Fprintf(os.Stderr, "error: node %s not connected\n", nodeID)
		os.Exit(1)
	}

	chunk, err := client.GetLogs(jobID, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(chunk.Data)
	if *follow && !chunk.EOF {
		followLogs(nodeID, jobID, chunk.Offset)
	}
}

func followLogs(nodeID, jobID string, offset int64) {
	client, ok := ClusterMgr.GetClient(nodeID)
	if !ok {
		return
	}

	for {
		time.Sleep(500 * time.Millisecond)
		chunk, err := client.GetLogs(jobID, offset)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nerror reading logs: %v\n", err)
			return
		}
		if len(chunk.Data) > 0 {
			fmt.Print(chunk.Data)
			offset = chunk.Offset
		}
		if chunk.EOF {
			return
		}
	}
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
