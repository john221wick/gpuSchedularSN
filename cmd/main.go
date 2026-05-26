package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/john221wick/gpuSchedularSN/internal/agent"
	"github.com/john221wick/gpuSchedularSN/internal/cli"
)

func main() {
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

	cli.Run(os.Args[1:])
}
