package main

import (
	"fmt"
	"log"
	"github.com/john221wick/gpuSchedularSN/internal/agent"
)

func main() {
	fmt.Println("Lets go")

	n, err := agent.Init()
	if err != nil {
		log.Fatal(err)
	}

	devices := agent.GetDevices()
	if n == 0 {
		fmt.Println("No GPU detected, using CPU only")
	} else if n == 1 {
		d := devices[0]
		fmt.Printf("Detected 1 GPU: %s  %dMB\n", d.Name, d.VRAMTotalMB)
	} else {
		fmt.Printf("Detected %d GPUs\n", n)
		for _, d := range devices {
			fmt.Printf("[%d] %s  %dMB  util:%.2f%%\n", d.ID, d.Name, d.VRAMTotalMB, d.UtilizationPct)
		}
		links := agent.GetTopology()
		if len(links) > 0 {
			fmt.Printf("\nTopology (%d links):\n", len(links))
			for _, l := range links {
				fmt.Printf("  GPU%d <-> GPU%d  %s  %.0f GB/s\n", l.GPUA, l.GPUB, l.Type, l.BandwidthGBps)
			}
		}
	}

	agent.Shutdown()
}
