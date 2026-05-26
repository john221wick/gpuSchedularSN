package main

import (
	"os"

	"github.com/john221wick/gpuSchedularSN/internal/cli"
)

func main() {
	cli.Run(os.Args[1:])
}
