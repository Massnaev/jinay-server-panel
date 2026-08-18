package powercontrol

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfilesApplyExpectedLimits(t *testing.T) {
	for _, test := range []struct{ profile, governor, maximum, noTurbo string }{
		{"eco", "schedutil", "2340000", "1"},
		{"balanced", "schedutil", "3600000", "0"},
		{"turbo", "performance", "3600000", "0"},
	} {
		t.Run(test.profile, func(t *testing.T) {
			root := fakeSysfs(t, 2)
			manager := &Manager{Root: root}
			result, err := manager.Apply(test.profile)
			if err != nil {
				t.Fatal(err)
			}
			if result.Profile != test.profile || result.Governor != test.governor || result.PoliciesChanged != 2 {
				t.Fatalf("unexpected result: %+v", result)
			}
			for index := 0; index < 2; index++ {
				policy := filepath.Join(root, "cpufreq", "policy"+string(rune('0'+index)))
				assertFile(t, filepath.Join(policy, "scaling_governor"), test.governor)
				assertFile(t, filepath.Join(policy, "scaling_max_freq"), test.maximum)
				assertFile(t, filepath.Join(policy, "scaling_min_freq"), "1200000")
			}
			assertFile(t, filepath.Join(root, "intel_pstate", "no_turbo"), test.noTurbo)
			assertFile(t, filepath.Join(root, "jinay-power-profile"), test.profile+"\n")
		})
	}
}

func TestApplyRollsBackEveryPolicyOnWriteFailure(t *testing.T) {
	root := fakeSysfs(t, 2)
	writes := 0
	failed := false
	manager := &Manager{Root: root, WriteFile: func(path string, content []byte, mode os.FileMode) error {
		writes++
		if !failed && writes == 5 {
			failed = true
			return errors.New("injected write failure")
		}
		return os.WriteFile(path, content, mode)
	}}
	result, err := manager.Apply("eco")
	if err == nil || !result.RollbackApplied || !strings.Contains(err.Error(), "previous settings restored") {
		t.Fatalf("expected successful rollback, result=%+v err=%v", result, err)
	}
	for index := 0; index < 2; index++ {
		policy := filepath.Join(root, "cpufreq", "policy"+string(rune('0'+index)))
		assertFile(t, filepath.Join(policy, "scaling_governor"), "schedutil")
		assertFile(t, filepath.Join(policy, "scaling_max_freq"), "3600000")
	}
	assertFile(t, filepath.Join(root, "intel_pstate", "no_turbo"), "0")
}

func TestEcoDoesNotRatchetWhenTurboIsAlreadyDisabled(t *testing.T) {
	for _, profile := range []string{"eco", "balanced"} {
		t.Run(profile, func(t *testing.T) {
			root := fakeSysfs(t, 2)
			if err := os.WriteFile(filepath.Join(root, "intel_pstate", "no_turbo"), []byte("1"), 0o644); err != nil {
				t.Fatal(err)
			}
			for index := 0; index < 2; index++ {
				policy := filepath.Join(root, "cpufreq", "policy"+string(rune('0'+index)))
				if err := os.WriteFile(filepath.Join(policy, "cpuinfo_max_freq"), []byte("2600000"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(policy, "scaling_max_freq"), []byte("2340000"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			manager := &Manager{Root: root, WriteFile: func(path string, content []byte, mode os.FileMode) error {
				if err := os.WriteFile(path, content, mode); err != nil {
					return err
				}
				if path == filepath.Join(root, "intel_pstate", "no_turbo") {
					hardwareMax := "3600000"
					if string(content) == "1" {
						hardwareMax = "2600000"
					}
					for index := 0; index < 2; index++ {
						policy := filepath.Join(root, "cpufreq", "policy"+string(rune('0'+index)))
						if err := os.WriteFile(filepath.Join(policy, "cpuinfo_max_freq"), []byte(hardwareMax), mode); err != nil {
							return err
						}
					}
				}
				return nil
			}}
			result, err := manager.Apply(profile)
			if err != nil {
				t.Fatal(err)
			}
			want := "2340000"
			if profile == "balanced" {
				want = "3600000"
			}
			for index := 0; index < 2; index++ {
				assertFile(t, filepath.Join(root, "cpufreq", "policy"+string(rune('0'+index)), "scaling_max_freq"), want)
			}
			if result.MaximumFrequencyMHz != map[string]float64{"eco": 2340, "balanced": 3600}[profile] {
				t.Fatalf("unexpected maximum: %+v", result)
			}
		})
	}
}

func TestClientRejectsUnknownProfileBeforeDial(t *testing.T) {
	_, err := (Client{Enabled: true, SocketPath: filepath.Join(t.TempDir(), "missing.sock")}).Apply(context.Background(), "custom")
	if err == nil || err.Error() != "unsupported power profile" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func fakeSysfs(t *testing.T, policies int) string {
	t.Helper()
	root := t.TempDir()
	write := func(path, value string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, "intel_pstate", "no_turbo"), "0")
	for index := 0; index < policies; index++ {
		policy := filepath.Join(root, "cpufreq", "policy"+string(rune('0'+index)))
		write(filepath.Join(policy, "scaling_governor"), "schedutil")
		write(filepath.Join(policy, "scaling_available_governors"), "conservative ondemand powersave performance schedutil")
		write(filepath.Join(policy, "scaling_min_freq"), "1200000")
		write(filepath.Join(policy, "scaling_max_freq"), "3600000")
		write(filepath.Join(policy, "cpuinfo_min_freq"), "1200000")
		write(filepath.Join(policy, "cpuinfo_max_freq"), "3600000")
	}
	return root
}

func assertFile(t *testing.T, path, expected string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != expected {
		t.Fatalf("%s: expected %q, got %q", path, expected, content)
	}
}
