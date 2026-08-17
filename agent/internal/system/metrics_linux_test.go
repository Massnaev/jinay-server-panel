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
