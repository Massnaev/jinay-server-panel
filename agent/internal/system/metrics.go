package system

import "time"

type Temperature struct {
	Label   string  `json:"label"`
	Celsius float64 `json:"celsius"`
}

type Network struct {
	ReceivedBytes    uint64 `json:"receivedBytes"`
	TransmittedBytes uint64 `json:"transmittedBytes"`
}

type SystemInfo struct {
	OSName             string  `json:"osName"`
	KernelVersion      string  `json:"kernelVersion"`
	Architecture       string  `json:"architecture"`
	CPUModel           string  `json:"cpuModel"`
	CPUSockets         int     `json:"cpuSockets"`
	CPUCores           int     `json:"cpuCores"`
	CPUThreads         int     `json:"cpuThreads"`
	CPUMaxFrequencyMHz float64 `json:"cpuMaxFrequencyMHz"`
}

type Metrics struct {
	Timestamp         time.Time     `json:"timestamp"`
	Hostname          string        `json:"hostname"`
	Platform          string        `json:"platform"`
	System            SystemInfo    `json:"system"`
	CPUCount          int           `json:"cpuCount"`
	CPUPercent        float64       `json:"cpuPercent"`
	Load              [3]float64    `json:"load"`
	MemoryTotalBytes  uint64        `json:"memoryTotalBytes"`
	MemoryUsedBytes   uint64        `json:"memoryUsedBytes"`
	DiskTotalBytes    uint64        `json:"diskTotalBytes"`
	DiskUsedBytes     uint64        `json:"diskUsedBytes"`
	Network           Network       `json:"network"`
	Temperatures      []Temperature `json:"temperatures"`
	UptimeSeconds     float64       `json:"uptimeSeconds"`
	CollectionWarning string        `json:"collectionWarning,omitempty"`
}

func percent(used, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return float64(used) / float64(total) * 100
}

func UsageSummary(metrics Metrics) map[string]float64 {
	maxTemperature := 0.0
	for _, temperature := range metrics.Temperatures {
		if temperature.Celsius > maxTemperature {
			maxTemperature = temperature.Celsius
		}
	}
	return map[string]float64{
		"cpuPercent":     metrics.CPUPercent,
		"memoryPercent":  percent(metrics.MemoryUsedBytes, metrics.MemoryTotalBytes),
		"diskPercent":    percent(metrics.DiskUsedBytes, metrics.DiskTotalBytes),
		"maxTemperature": maxTemperature,
	}
}
