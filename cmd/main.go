package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/john221wick/gpuSchedularSN/internal/agent"
	"github.com/john221wick/gpuSchedularSN/internal/agentserver"
	"github.com/john221wick/gpuSchedularSN/internal/cli"
)

func main() {
	agentMode := false
	port := 9712

	// Parse --agent and --port before passing rest to cli.Run
	var cliArgs []string
	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--agent":
			agentMode = true
		case "--port":
			if i+1 < len(os.Args) {
				if n, err := strconv.Atoi(os.Args[i+1]); err == nil {
					port = n
				}
				i++
			}
		default:
			cliArgs = append(cliArgs, os.Args[i])
		}
	}

	if agentMode {
		runAgent(port)
		return
	}

	// Existing CLI path
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nShutting down...")
		if cli.GlobalState != nil {
			cli.GlobalState.Stop()
		}
		agent.Shutdown()
		os.Exit(0)
	}()

	cli.Run(cliArgs)
}

func runAgent(port int) {
	srv, err := agentserver.NewAgentServer()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start agent: %v\n", err)
		os.Exit(1)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nAgent shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
		os.Exit(0)
	}()

	addr := fmt.Sprintf(":%d", port)
	if err := srv.ListenAndServe(addr); err != nil {
		fmt.Fprintf(os.Stderr, "Agent server error: %v\n", err)
		os.Exit(1)
	}
}
