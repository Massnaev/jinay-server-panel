//go:build linux

package system

import (
	"testing"
	"time"
)

func TestParseOSRelease(t *testing.T) {
	content := "NAME=Ubuntu\nPRETTY_NAME=\"Ubuntu 26.04 LTS\"\n"
	if got := parseOSRelease(content); got != "Ubuntu 26.04 LTS" {
		t.Fatalf("unexpected OS name %q", got)
	}
}

func TestParseDualSocketCPUInfo(t *testing.T) {
	content := `processor : 0
model name : Intel(R) Xeon(R) CPU E5-2689 0 @ 2.60GHz
physical id : 0
core id : 0

processor : 1
model name : Intel(R) Xeon(R) CPU E5-2689 0 @ 2.60GHz
physical id : 0
core id : 1

processor : 2
model name : Intel(R) Xeon(R) CPU E5-2689 0 @ 2.60GHz
physical id : 1
core id : 0

processor : 3
model name : Intel(R) Xeon(R) CPU E5-2689 0 @ 2.60GHz
physical id : 1
core id : 1`

	info := parseCPUInfo(content)
	if info.CPUModel != "Intel(R) Xeon(R) CPU E5-2689 0 @ 2.60GHz" {
		t.Fatalf("unexpected model %q", info.CPUModel)
	}
	if info.CPUSockets != 2 || info.CPUCores != 4 {
		t.Fatalf("unexpected topology: sockets=%d cores=%d", info.CPUSockets, info.CPUCores)
	}
	if len(info.Processors) != 2 {
		t.Fatalf("expected two physical processors, got %+v", info.Processors)
	}
	if info.Processors[0].SocketID != "0" || info.Processors[0].PhysicalCores != 2 || info.Processors[0].LogicalThreads != 2 {
		t.Fatalf("unexpected first processor: %+v", info.Processors[0])
	}
	if info.Processors[1].SocketID != "1" || info.Processors[1].PhysicalCores != 2 || info.Processors[1].LogicalThreads != 2 {
		t.Fatalf("unexpected second processor: %+v", info.Processors[1])
	}
	if len(info.Processors[0].LogicalCPUIds) != 2 || info.Processors[0].LogicalCPUIds[0] != 0 || info.Processors[1].LogicalCPUIds[0] != 2 {
		t.Fatalf("unexpected logical CPU mapping: %+v", info.Processors)
	}
}

func TestParseCPUStatsAndSocketUtilization(t *testing.T) {
	first, err := parseCPUStats("cpu  100 0 50 850 0 0 0 0\ncpu0 50 0 25 425 0 0 0 0\ncpu1 50 0 25 425 0 0 0 0\n")
	if err != nil {
		t.Fatal(err)
	}
	second, err := parseCPUStats("cpu  130 0 60 910 0 0 0 0\ncpu0 70 0 30 450 0 0 0 0\ncpu1 60 0 30 460 0 0 0 0\n")
	if err != nil {
		t.Fatal(err)
	}
	if got := cpuUtilization(first.aggregate, second.aggregate); got != 40 {
		t.Fatalf("unexpected aggregate utilization %.2f", got)
	}
	firstSocket := addCPUTimes(first.logical[0], first.logical[1])
	secondSocket := addCPUTimes(second.logical[0], second.logical[1])
	if got := cpuUtilization(firstSocket, secondSocket); got != 40 {
		t.Fatalf("unexpected socket utilization %.2f", got)
	}
}

func TestCounterRate(t *testing.T) {
	if got := counterRate(1_000, 3_000, 2*time.Second); got != 1_000 {
		t.Fatalf("unexpected counter rate %.2f", got)
	}
	if got := counterRate(3_000, 1_000, time.Second); got != 0 {
		t.Fatalf("counter reset should return zero, got %.2f", got)
	}
}

func TestParsePCIDeviceName(t *testing.T) {
	content := "# PCI IDs\n10de  NVIDIA Corporation\n\t0615  G92 [GeForce GTS 250]\n\t\t1462 1542  Board\n1002  AMD\n\t73bf  Navi 21\n"
	if got := parsePCIDeviceName(content, "10de", "0615"); got != "G92 [GeForce GTS 250]" {
		t.Fatalf("unexpected PCI model %q", got)
	}
	if got := parsePCIDeviceName(content, "10de", "ffff"); got != "" {
		t.Fatalf("unexpected unknown PCI model %q", got)
	}
}

func TestParseGPUUEvent(t *testing.T) {
	values := parseKeyValues("DRIVER=nouveau\nPCI_ID=10DE:0615\nPCI_SLOT_NAME=0000:03:00.0\n")
	if values["DRIVER"] != "nouveau" || values["PCI_ID"] != "10DE:0615" {
		t.Fatalf("unexpected uevent values: %+v", values)
	}
	if got := pciVendorName("10de"); got != "NVIDIA" {
		t.Fatalf("unexpected vendor %q", got)
	}
}

func TestParseMemoryAndSwap(t *testing.T) {
	info := parseMemoryInfo("MemTotal: 32768 kB\nMemAvailable: 24576 kB\nSwapTotal: 8192 kB\nSwapFree: 6144 kB\n")
	if info.Total != 32768*1024 || info.Used != 8192*1024 {
		t.Fatalf("unexpected memory values: %+v", info)
	}
	if info.SwapTotal != 8192*1024 || info.SwapUsed != 2048*1024 {
		t.Fatalf("unexpected swap values: %+v", info)
	}
}

func TestPowerValueSummary(t *testing.T) {
	if got := joinedState([]string{"schedutil", "schedutil"}); got != "schedutil" {
		t.Fatalf("unexpected governor %q", got)
	}
	if got := joinedState([]string{"powersave", "performance"}); got != "mixed" {
		t.Fatalf("expected mixed state, got %q", got)
	}
	if got := average([]float64{1200, 2400, 3600}); got != 2400 {
		t.Fatalf("unexpected average %f", got)
	}
}
