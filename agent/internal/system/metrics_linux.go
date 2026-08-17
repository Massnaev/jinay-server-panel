//go:build linux

package system

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type cpuTimes struct {
	total uint64
	idle  uint64
}

func ReadMetrics() (Metrics, error) {
	hostname, _ := os.Hostname()
	metrics := Metrics{
		Timestamp:    time.Now().UTC(),
		Hostname:     hostname,
		Platform:     "linux",
		CPUCount:     runtime.NumCPU(),
		System:       readSystemInfo(),
		Temperatures: []Temperature{},
	}

	first, err := readCPUStats()
	if err != nil {
		return metrics, err
	}
	firstNetwork := readNetwork()
	sampleStarted := time.Now()
	time.Sleep(120 * time.Millisecond)
	second, err := readCPUStats()
	if err != nil {
		return metrics, err
	}
	secondNetwork := readNetwork()
	sampleDuration := time.Since(sampleStarted)
	metrics.CPUPercent = cpuUtilization(first.aggregate, second.aggregate)
	for index := range metrics.System.Processors {
		processor := &metrics.System.Processors[index]
		var firstSocket, secondSocket cpuTimes
		for _, logicalID := range processor.LogicalCPUIds {
			firstSocket = addCPUTimes(firstSocket, first.logical[logicalID])
			secondSocket = addCPUTimes(secondSocket, second.logical[logicalID])
		}
		processor.UtilizationPercent = cpuUtilization(firstSocket, secondSocket)
	}

	if load, err := os.ReadFile("/proc/loadavg"); err == nil {
		fields := strings.Fields(string(load))
		for i := 0; i < 3 && i < len(fields); i++ {
			metrics.Load[i], _ = strconv.ParseFloat(fields[i], 64)
		}
	}
	if uptime, err := os.ReadFile("/proc/uptime"); err == nil {
		fields := strings.Fields(string(uptime))
		if len(fields) > 0 {
			metrics.UptimeSeconds, _ = strconv.ParseFloat(fields[0], 64)
		}
	}
	memory := readMemory()
	metrics.MemoryTotalBytes, metrics.MemoryUsedBytes = memory.Total, memory.Used
	metrics.SwapTotalBytes, metrics.SwapUsedBytes = memory.SwapTotal, memory.SwapUsed
	metrics.DiskTotalBytes, metrics.DiskUsedBytes = readDisk()
	metrics.Network = secondNetwork
	metrics.Network.ReceiveBytesPerSecond = counterRate(firstNetwork.ReceivedBytes, metrics.Network.ReceivedBytes, sampleDuration)
	metrics.Network.TransmitBytesPerSecond = counterRate(firstNetwork.TransmittedBytes, metrics.Network.TransmittedBytes, sampleDuration)
	metrics.Temperatures = readTemperatures()
	metrics.Fans = readFans()
	metrics.Power = readPowerInfo()
	return metrics, nil
}

func readSystemInfo() SystemInfo {
	info := SystemInfo{
		OSName:       "Linux",
		Architecture: runtime.GOARCH,
		CPUThreads:   runtime.NumCPU(),
	}
	if content, err := os.ReadFile("/etc/os-release"); err == nil {
		info.OSName = parseOSRelease(string(content))
	}
	if content, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
		info.KernelVersion = strings.TrimSpace(string(content))
	}
	if content, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		parsed := parseCPUInfo(string(content))
		info.CPUModel = parsed.CPUModel
		info.CPUSockets = parsed.CPUSockets
		info.CPUCores = parsed.CPUCores
		info.Processors = parsed.Processors
	}
	if info.CPUSockets == 0 {
		info.CPUSockets = 1
	}
	if info.CPUCores == 0 {
		info.CPUCores = info.CPUThreads
	}
	if content, err := os.ReadFile("/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq"); err == nil {
		kilohertz, _ := strconv.ParseFloat(strings.TrimSpace(string(content)), 64)
		info.CPUMaxFrequencyMHz = kilohertz / 1000
	}
	info.GPUs = readGPUs()
	return info
}

func parseOSRelease(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if !strings.HasPrefix(line, "PRETTY_NAME=") {
			continue
		}
		return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"'")
	}
	return "Linux"
}

func parseCPUInfo(content string) SystemInfo {
	info := SystemInfo{}
	type socketData struct {
		model      string
		cores      map[string]struct{}
		threads    int
		logicalIDs []int
	}
	sockets := make(map[string]*socketData)
	fallbackSocket := "0"
	for _, block := range strings.Split(strings.TrimSpace(content), "\n\n") {
		values := make(map[string]string)
		for _, line := range strings.Split(block, "\n") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				values[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
		if info.CPUModel == "" {
			info.CPUModel = values["model name"]
		}
		physicalID := values["physical id"]
		if physicalID == "" {
			physicalID = fallbackSocket
		}
		socket := sockets[physicalID]
		if socket == nil {
			socket = &socketData{model: values["model name"], cores: make(map[string]struct{})}
			sockets[physicalID] = socket
		}
		socket.threads++
		if logicalID, err := strconv.Atoi(values["processor"]); err == nil {
			socket.logicalIDs = append(socket.logicalIDs, logicalID)
		}
		if coreID := values["core id"]; coreID != "" {
			socket.cores[coreID] = struct{}{}
		}
	}
	info.CPUSockets = len(sockets)
	ids := make([]string, 0, len(sockets))
	for id := range sockets {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		left, leftErr := strconv.Atoi(ids[i])
		right, rightErr := strconv.Atoi(ids[j])
		if leftErr == nil && rightErr == nil {
			return left < right
		}
		return ids[i] < ids[j]
	})
	for _, id := range ids {
		socket := sockets[id]
		coreCount := len(socket.cores)
		if coreCount == 0 {
			coreCount = socket.threads
		}
		info.CPUCores += coreCount
		info.Processors = append(info.Processors, ProcessorInfo{
			SocketID: id, Model: socket.model, PhysicalCores: coreCount, LogicalThreads: socket.threads, LogicalCPUIds: socket.logicalIDs,
		})
	}
	return info
}

func readGPUs() []GPUInfo {
	paths, _ := filepath.Glob("/sys/class/drm/card[0-9]*/device/uevent")
	gpus := make([]GPUInfo, 0, len(paths))
	for _, path := range paths {
		values := parseKeyValues(readTrimmed(path))
		pciID := strings.SplitN(values["PCI_ID"], ":", 2)
		if len(pciID) != 2 {
			continue
		}
		vendorID, deviceID := strings.ToLower(pciID[0]), strings.ToLower(pciID[1])
		vendor := pciVendorName(vendorID)
		model := lookupPCIDevice(vendorID, deviceID)
		if model == "" {
			model = fmt.Sprintf("%s GPU [%s:%s]", vendor, vendorID, deviceID)
		}
		gpus = append(gpus, GPUInfo{
			Card: filepath.Base(filepath.Dir(filepath.Dir(path))), Model: model, Vendor: vendor,
			VendorID: vendorID, DeviceID: deviceID, PCISlot: values["PCI_SLOT_NAME"], Driver: values["DRIVER"],
			TemperatureCelsius: readGPUTemperature(filepath.Dir(path)),
		})
	}
	sort.Slice(gpus, func(i, j int) bool { return gpus[i].Card < gpus[j].Card })
	return gpus
}

func parseKeyValues(content string) map[string]string {
	values := make(map[string]string)
	for _, line := range strings.Split(content, "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			values[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return values
}

func pciVendorName(id string) string {
	switch strings.ToLower(id) {
	case "10de":
		return "NVIDIA"
	case "1002", "1022":
		return "AMD"
	case "8086":
		return "Intel"
	default:
		return "PCI"
	}
}

func lookupPCIDevice(vendorID, deviceID string) string {
	for _, path := range []string{"/usr/share/misc/pci.ids", "/usr/share/hwdata/pci.ids", "/usr/share/pci.ids"} {
		content, err := os.ReadFile(path)
		if err == nil {
			return parsePCIDeviceName(string(content), vendorID, deviceID)
		}
	}
	return ""
}

func parsePCIDeviceName(content, vendorID, deviceID string) string {
	inVendor := false
	for _, line := range strings.Split(content, "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, "\t") {
			fields := strings.Fields(line)
			inVendor = len(fields) >= 2 && strings.EqualFold(fields[0], vendorID)
			continue
		}
		if !inVendor || strings.HasPrefix(line, "\t\t") {
			continue
		}
		trimmed := strings.TrimSpace(line)
		fields := strings.Fields(trimmed)
		if len(fields) >= 2 && strings.EqualFold(fields[0], deviceID) {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, fields[0]))
		}
	}
	return ""
}

func readGPUTemperature(devicePath string) float64 {
	paths, _ := filepath.Glob(filepath.Join(devicePath, "hwmon", "hwmon*", "temp*_input"))
	for _, path := range paths {
		milliCelsius, err := strconv.ParseFloat(readTrimmed(path), 64)
		if err == nil && milliCelsius > 0 && milliCelsius <= 150_000 {
			return milliCelsius / 1000
		}
	}
	return 0
}

type cpuStats struct {
	aggregate cpuTimes
	logical   map[int]cpuTimes
}

func readCPUStats() (cpuStats, error) {
	content, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuStats{}, fmt.Errorf("read CPU statistics: %w", err)
	}
	return parseCPUStats(string(content))
}

func parseCPUStats(content string) (cpuStats, error) {
	stats := cpuStats{logical: make(map[int]cpuTimes)}
	for _, line := range strings.Split(content, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 || !strings.HasPrefix(fields[0], "cpu") {
			continue
		}
		times, err := parseCPUTimes(fields)
		if err != nil {
			return cpuStats{}, err
		}
		if fields[0] == "cpu" {
			stats.aggregate = times
			continue
		}
		logicalID, err := strconv.Atoi(strings.TrimPrefix(fields[0], "cpu"))
		if err == nil {
			stats.logical[logicalID] = times
		}
	}
	if stats.aggregate.total == 0 {
		return cpuStats{}, fmt.Errorf("unexpected /proc/stat CPU rows")
	}
	return stats, nil
}

func parseCPUTimes(fields []string) (cpuTimes, error) {
	var values []uint64
	for _, field := range fields[1:] {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return cpuTimes{}, fmt.Errorf("parse CPU statistics: %w", err)
		}
		values = append(values, value)
	}
	var total uint64
	for _, value := range values {
		total += value
	}
	idle := values[3]
	if len(values) > 4 {
		idle += values[4]
	}
	return cpuTimes{total: total, idle: idle}, nil
}

func addCPUTimes(left, right cpuTimes) cpuTimes {
	return cpuTimes{total: left.total + right.total, idle: left.idle + right.idle}
}

func cpuUtilization(first, second cpuTimes) float64 {
	if second.total <= first.total || second.idle < first.idle {
		return 0
	}
	totalDelta := second.total - first.total
	idleDelta := second.idle - first.idle
	if idleDelta > totalDelta {
		return 0
	}
	return float64(totalDelta-idleDelta) / float64(totalDelta) * 100
}

func counterRate(first, second uint64, duration time.Duration) float64 {
	if second < first || duration <= 0 {
		return 0
	}
	return float64(second-first) / duration.Seconds()
}

type memoryInfo struct {
	Total     uint64
	Used      uint64
	SwapTotal uint64
	SwapUsed  uint64
}

func readMemory() memoryInfo {
	content, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return memoryInfo{}
	}
	return parseMemoryInfo(string(content))
}

func parseMemoryInfo(content string) memoryInfo {
	values := make(map[string]uint64)
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err == nil {
			values[strings.TrimSuffix(fields[0], ":")] = value * 1024
		}
	}
	total := values["MemTotal"]
	available := values["MemAvailable"]
	if available > total {
		available = total
	}
	swapTotal := values["SwapTotal"]
	swapFree := values["SwapFree"]
	if swapFree > swapTotal {
		swapFree = swapTotal
	}
	return memoryInfo{Total: total, Used: total - available, SwapTotal: swapTotal, SwapUsed: swapTotal - swapFree}
}

func readDisk() (uint64, uint64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err != nil {
		return 0, 0
	}
	total := stat.Blocks * uint64(stat.Bsize)
	available := stat.Bavail * uint64(stat.Bsize)
	return total, total - available
}

func readNetwork() Network {
	file, err := os.Open("/proc/net/dev")
	if err != nil {
		return Network{}
	}
	defer file.Close()
	var network Network
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if strings.TrimSpace(parts[0]) == "lo" {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) < 9 {
			continue
		}
		received, _ := strconv.ParseUint(fields[0], 10, 64)
		transmitted, _ := strconv.ParseUint(fields[8], 10, 64)
		network.ReceivedBytes += received
		network.TransmittedBytes += transmitted
	}
	return network
}

func readTemperatures() []Temperature {
	paths, _ := filepath.Glob("/sys/class/hwmon/hwmon*/temp*_input")
	temperatures := make([]Temperature, 0, len(paths))
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		milliCelsius, err := strconv.ParseFloat(strings.TrimSpace(string(content)), 64)
		if err != nil || milliCelsius <= 0 || milliCelsius > 150_000 {
			continue
		}
		root := filepath.Dir(path)
		chip := readTrimmed(filepath.Join(root, "name"))
		label := filepath.Base(strings.TrimSuffix(path, "_input"))
		if content, err := os.ReadFile(strings.TrimSuffix(path, "_input") + "_label"); err == nil {
			label = strings.TrimSpace(string(content))
		}
		if chip != "" {
			label = chip + " · " + label
		}
		temperatures = append(temperatures, Temperature{Label: label, Celsius: milliCelsius / 1000})
	}
	return temperatures
}

func readFans() []Fan {
	paths, _ := filepath.Glob("/sys/class/hwmon/hwmon*/fan*_input")
	fans := make([]Fan, 0, len(paths))
	for _, path := range paths {
		rpm, err := strconv.ParseFloat(readTrimmed(path), 64)
		if err != nil || rpm < 0 {
			continue
		}
		root := filepath.Dir(path)
		base := strings.TrimSuffix(filepath.Base(path), "_input")
		index := strings.TrimPrefix(base, "fan")
		label := readTrimmed(filepath.Join(root, base+"_label"))
		if label == "" {
			label = "Fan " + index
		}
		if chip := readTrimmed(filepath.Join(root, "name")); chip != "" {
			label = chip + " · " + label
		}
		_, pwmErr := os.Stat(filepath.Join(root, "pwm"+index))
		fans = append(fans, Fan{Label: label, RPM: rpm, PWMDetected: pwmErr == nil})
	}
	return fans
}

func readPowerInfo() PowerInfo {
	governors := readGlobValues("/sys/devices/system/cpu/cpu[0-9]*/cpufreq/scaling_governor")
	drivers := readGlobValues("/sys/devices/system/cpu/cpu[0-9]*/cpufreq/scaling_driver")
	available := readWords("/sys/devices/system/cpu/cpu0/cpufreq/scaling_available_governors")
	current := readGlobKilohertz("/sys/devices/system/cpu/cpu[0-9]*/cpufreq/scaling_cur_freq")
	minimum := readKilohertz("/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_min_freq")
	maximum := readKilohertz("/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq")
	return PowerInfo{
		Governor:              joinedState(governors),
		AvailableGovernors:    available,
		Driver:                joinedState(drivers),
		CurrentFrequencyMHz:   average(current),
		MinimumFrequencyMHz:   minimum,
		MaximumFrequencyMHz:   maximum,
		PlatformProfile:       readTrimmed("/sys/firmware/acpi/platform_profile"),
		AvailableProfiles:     readWords("/sys/firmware/acpi/platform_profile_choices"),
		ControlSupported:      false,
		ControlDisabledReason: "Read-only detection is enabled; safe profile switching requires a validated privileged helper and rollback.",
	}
}

func readGlobValues(pattern string) []string {
	paths, _ := filepath.Glob(pattern)
	values := make([]string, 0, len(paths))
	for _, path := range paths {
		if value := readTrimmed(path); value != "" {
			values = append(values, value)
		}
	}
	return values
}

func joinedState(values []string) string {
	if len(values) == 0 {
		return "unavailable"
	}
	first := values[0]
	for _, value := range values[1:] {
		if value != first {
			return "mixed"
		}
	}
	return first
}

func readWords(path string) []string {
	value := readTrimmed(path)
	if value == "" {
		return []string{}
	}
	return strings.Fields(value)
}

func readGlobKilohertz(pattern string) []float64 {
	paths, _ := filepath.Glob(pattern)
	values := make([]float64, 0, len(paths))
	for _, path := range paths {
		if value := readKilohertz(path); value > 0 {
			values = append(values, value)
		}
	}
	return values
}

func readKilohertz(path string) float64 {
	kilohertz, _ := strconv.ParseFloat(readTrimmed(path), 64)
	return kilohertz / 1000
}

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values))
}

func readTrimmed(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(content))
}
