package system

import "time"

type Temperature struct {
	Label   string  `json:"label"`
	Celsius float64 `json:"celsius"`
}

type Fan struct {
	Label       string  `json:"label"`
	RPM         float64 `json:"rpm"`
	PWMDetected bool    `json:"pwmDetected"`
}

type PowerInfo struct {
	Governor              string   `json:"governor"`
	AvailableGovernors    []string `json:"availableGovernors"`
	Driver                string   `json:"driver"`
	CurrentFrequencyMHz   float64  `json:"currentFrequencyMHz"`
	MinimumFrequencyMHz   float64  `json:"minimumFrequencyMHz"`
	MaximumFrequencyMHz   float64  `json:"maximumFrequencyMHz"`
	PlatformProfile       string   `json:"platformProfile"`
	AvailableProfiles     []string `json:"availableProfiles"`
	ControlSupported      bool     `json:"controlSupported"`
	ControlDisabledReason string   `json:"controlDisabledReason"`
}

type Network struct {
	ReceivedBytes          uint64  `json:"receivedBytes"`
	TransmittedBytes       uint64  `json:"transmittedBytes"`
	ReceiveBytesPerSecond  float64 `json:"receiveBytesPerSecond"`
	TransmitBytesPerSecond float64 `json:"transmitBytesPerSecond"`
}

type ProcessorInfo struct {
	SocketID           string  `json:"socketId"`
	Model              string  `json:"model"`
	PhysicalCores      int     `json:"physicalCores"`
	LogicalThreads     int     `json:"logicalThreads"`
	UtilizationPercent float64 `json:"utilizationPercent"`
	LogicalCPUIds      []int   `json:"-"`
}

type GPUInfo struct {
	Card               string  `json:"card"`
	Model              string  `json:"model"`
	Vendor             string  `json:"vendor"`
	VendorID           string  `json:"vendorId"`
	DeviceID           string  `json:"deviceId"`
	PCISlot            string  `json:"pciSlot"`
	Driver             string  `json:"driver"`
	TemperatureCelsius float64 `json:"temperatureCelsius,omitempty"`
}

type SystemInfo struct {
	OSName             string          `json:"osName"`
	KernelVersion      string          `json:"kernelVersion"`
	Architecture       string          `json:"architecture"`
	CPUModel           string          `json:"cpuModel"`
	CPUSockets         int             `json:"cpuSockets"`
	CPUCores           int             `json:"cpuCores"`
	CPUThreads         int             `json:"cpuThreads"`
	CPUMaxFrequencyMHz float64         `json:"cpuMaxFrequencyMHz"`
	Processors         []ProcessorInfo `json:"processors"`
	GPUs               []GPUInfo       `json:"gpus"`
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
	SwapTotalBytes    uint64        `json:"swapTotalBytes"`
	SwapUsedBytes     uint64        `json:"swapUsedBytes"`
	DiskTotalBytes    uint64        `json:"diskTotalBytes"`
	DiskUsedBytes     uint64        `json:"diskUsedBytes"`
	Network           Network       `json:"network"`
	Temperatures      []Temperature `json:"temperatures"`
	Fans              []Fan         `json:"fans"`
	Power             PowerInfo     `json:"power"`
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
		"swapPercent":    percent(metrics.SwapUsedBytes, metrics.SwapTotalBytes),
		"diskPercent":    percent(metrics.DiskUsedBytes, metrics.DiskTotalBytes),
		"maxTemperature": maxTemperature,
	}
}
