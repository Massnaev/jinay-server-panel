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
		Timestamp: time.Now().UTC(),
		Hostname:  hostname,
		Platform:  runtime.GOOS,
		CPUCount:  runtime.NumCPU(),
		System: SystemInfo{
			OSName:       runtime.GOOS,
			Architecture: runtime.GOARCH,
			CPUSockets:   1,
			CPUCores:     runtime.NumCPU(),
			CPUThreads:   runtime.NumCPU(),
			Processors:   []ProcessorInfo{{SocketID: "0", PhysicalCores: runtime.NumCPU(), LogicalThreads: runtime.NumCPU()}},
			GPUs:         []GPUInfo{},
		},
		MemoryTotalBytes: memory.Sys,
		MemoryUsedBytes:  memory.Alloc,
		Temperatures:     []Temperature{},
		Fans:             []Fan{},
		StorageDevices:   []StorageDevice{},
		Power: PowerInfo{
			Governor:              "unavailable",
			Driver:                "unavailable",
			AvailableGovernors:    []string{},
			AvailableProfiles:     []string{},
			ControlDisabledReason: "Power telemetry is available on Linux only.",
		},
		CollectionWarning: "Full host telemetry is available on Linux only.",
	}, nil
}
