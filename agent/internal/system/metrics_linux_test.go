//go:build linux

package system

import "testing"

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
