//go:build linux

package system

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
		Temperatures: []Temperature{},
	}

	first, err := readCPUTimes()
	if err != nil {
		return metrics, err
	}
	time.Sleep(120 * time.Millisecond)
	second, err := readCPUTimes()
	if err != nil {
		return metrics, err
	}
	totalDelta := second.total - first.total
	idleDelta := second.idle - first.idle
	if totalDelta > 0 {
		metrics.CPUPercent = float64(totalDelta-idleDelta) / float64(totalDelta) * 100
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
	metrics.MemoryTotalBytes, metrics.MemoryUsedBytes = readMemory()
	metrics.DiskTotalBytes, metrics.DiskUsedBytes = readDisk()
	metrics.Network = readNetwork()
	metrics.Temperatures = readTemperatures()
	return metrics, nil
}

func readCPUTimes() (cpuTimes, error) {
	content, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuTimes{}, fmt.Errorf("read CPU statistics: %w", err)
	}
	line := strings.SplitN(string(content), "\n", 2)[0]
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return cpuTimes{}, fmt.Errorf("unexpected /proc/stat CPU row")
	}
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

func readMemory() (uint64, uint64) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	defer file.Close()
	values := make(map[string]uint64)
	scanner := bufio.NewScanner(file)
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
	return total, total - available
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
		label := filepath.Base(strings.TrimSuffix(path, "_input"))
		if content, err := os.ReadFile(strings.TrimSuffix(path, "_input") + "_label"); err == nil {
			label = strings.TrimSpace(string(content))
		}
		temperatures = append(temperatures, Temperature{Label: label, Celsius: milliCelsius / 1000})
	}
	return temperatures
}
