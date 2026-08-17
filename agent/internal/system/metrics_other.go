//go:build !linux

package system

import (
	"os"
	"runtime"
	"time"
)

func ReadMetrics() (Metrics, error) {
	hostname, _ := os.Hostname()
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	return Metrics{
		Timestamp:         time.Now().UTC(),
		Hostname:          hostname,
		Platform:          runtime.GOOS,
		CPUCount:          runtime.NumCPU(),
		MemoryTotalBytes:  memory.Sys,
		MemoryUsedBytes:   memory.Alloc,
		Temperatures:      []Temperature{},
		CollectionWarning: "Full host telemetry is available on Linux only.",
	}, nil
}
