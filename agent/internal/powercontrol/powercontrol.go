package powercontrol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DefaultSocketPath = "/run/serverpanel-power/power.sock"
	DefaultSysfsRoot  = "/sys/devices/system/cpu"
	DefaultStatePath  = "/var/lib/serverpanel-power/profile"
	EcoMaximumPercent = 65
)

var ErrDisabled = errors.New("power profile control is disabled by configuration")

type Result struct {
	Profile             string  `json:"profile"`
	Governor            string  `json:"governor"`
	MaximumFrequencyMHz float64 `json:"maximumFrequencyMHz"`
	TurboAllowed        bool    `json:"turboAllowed"`
	PoliciesChanged     int     `json:"policiesChanged"`
	RollbackApplied     bool    `json:"rollbackApplied"`
}

type request struct {
	Profile string `json:"profile"`
}

type response struct {
	Result *Result `json:"result,omitempty"`
	Error  string  `json:"error,omitempty"`
}

type policySnapshot struct {
	path        string
	governor    string
	minimumKHz  int64
	maximumKHz  int64
	hardwareMin int64
	hardwareMax int64
	available   map[string]bool
}

type Manager struct {
	Root      string
	StatePath string
	WriteFile func(string, []byte, os.FileMode) error
	mu        sync.Mutex
}

func ValidateProfile(profile string) error {
	switch profile {
	case "eco", "balanced", "turbo":
		return nil
	default:
		return errors.New("unsupported power profile")
	}
}

func (m *Manager) Apply(profile string) (Result, error) {
	if err := ValidateProfile(profile); err != nil {
		return Result{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	root := m.Root
	if root == "" {
		root = DefaultSysfsRoot
	}
	paths, err := filepath.Glob(filepath.Join(root, "cpufreq", "policy*"))
	if err != nil || len(paths) == 0 {
		return Result{}, errors.New("CPU frequency policies are unavailable")
	}
	sort.Strings(paths)
	snapshots := make([]policySnapshot, 0, len(paths))
	for _, path := range paths {
		snapshot, readErr := readPolicy(path)
		if readErr != nil {
			return Result{}, readErr
		}
		snapshots = append(snapshots, snapshot)
	}

	governor, err := selectGovernor(profile, snapshots)
	if err != nil {
		return Result{}, err
	}
	noTurboPath := filepath.Join(root, "intel_pstate", "no_turbo")
	noTurboOriginal, noTurboAvailable := readOptionalInt(noTurboPath)
	turboAllowed := true
	if noTurboAvailable && profile == "eco" {
		turboAllowed = false
	}
	result := Result{Profile: profile, Governor: governor, TurboAllowed: turboAllowed, PoliciesChanged: len(snapshots)}

	rollback := func(cause error) (Result, error) {
		result.RollbackApplied = true
		rollbackErr := m.restore(snapshots, noTurboPath, noTurboOriginal, noTurboAvailable)
		if rollbackErr != nil {
			return result, fmt.Errorf("apply power profile: %w; rollback failed: %v", cause, rollbackErr)
		}
		return result, fmt.Errorf("apply power profile: %w; previous settings restored", cause)
	}

	// On Intel, cpuinfo_max_freq may hide the turbo ceiling while no_turbo=1.
	// Temporarily expose it so repeated Eco applications cannot ratchet the cap
	// downward and Balanced can always restore the complete hardware range.
	if noTurboAvailable && noTurboOriginal == 1 {
		if err := m.writeInt(noTurboPath, 0); err != nil {
			return rollback(err)
		}
		for index := range snapshots {
			hardwareMax, readErr := readPositiveInt(filepath.Join(snapshots[index].path, "cpuinfo_max_freq"), "cpuinfo_max_freq")
			if readErr != nil {
				return rollback(readErr)
			}
			snapshots[index].hardwareMax = hardwareMax
		}
	}

	if noTurboAvailable {
		value := int64(0)
		if !turboAllowed {
			value = 1
		}
		if err := m.writeInt(noTurboPath, value); err != nil {
			return rollback(err)
		}
	}

	for _, snapshot := range snapshots {
		targetMax := snapshot.hardwareMax
		if profile == "eco" {
			targetMax = snapshot.hardwareMax * EcoMaximumPercent / 100
			if targetMax < snapshot.hardwareMin {
				targetMax = snapshot.hardwareMin
			}
		}
		if err := m.writeInt(filepath.Join(snapshot.path, "scaling_min_freq"), snapshot.hardwareMin); err != nil {
			return rollback(err)
		}
		if err := m.writeInt(filepath.Join(snapshot.path, "scaling_max_freq"), targetMax); err != nil {
			return rollback(err)
		}
		if err := m.writeString(filepath.Join(snapshot.path, "scaling_governor"), governor); err != nil {
			return rollback(err)
		}
		result.MaximumFrequencyMHz += float64(targetMax) / 1000
	}
	result.MaximumFrequencyMHz /= float64(len(snapshots))

	if err := verifyApplied(snapshots, profile, governor); err != nil {
		return rollback(err)
	}
	if noTurboAvailable {
		expected := int64(0)
		if !turboAllowed {
			expected = 1
		}
		if actual, ok := readOptionalInt(noTurboPath); !ok || actual != expected {
			return rollback(errors.New("turbo policy verification failed"))
		}
	}
	if err := m.persistProfile(profile); err != nil {
		return rollback(err)
	}
	return result, nil
}

func (m *Manager) persistedProfile() (string, bool) {
	content, err := os.ReadFile(m.statePath())
	if err != nil {
		return "", false
	}
	profile := strings.TrimSpace(string(content))
	return profile, ValidateProfile(profile) == nil
}

func (m *Manager) persistProfile(profile string) error {
	path := m.statePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("persist power profile: %w", err)
	}
	if err := os.WriteFile(path, []byte(profile+"\n"), 0o600); err != nil {
		return fmt.Errorf("persist power profile: %w", err)
	}
	return nil
}

func (m *Manager) statePath() string {
	if m.StatePath != "" {
		return m.StatePath
	}
	if m.Root != "" {
		return filepath.Join(m.Root, "jinay-power-profile")
	}
	return DefaultStatePath
}

func readPolicy(path string) (policySnapshot, error) {
	minimum, err := readPositiveInt(filepath.Join(path, "scaling_min_freq"), "scaling_min_freq")
	if err != nil {
		return policySnapshot{}, err
	}
	maximum, err := readPositiveInt(filepath.Join(path, "scaling_max_freq"), "scaling_max_freq")
	if err != nil {
		return policySnapshot{}, err
	}
	hardwareMin, err := readPositiveInt(filepath.Join(path, "cpuinfo_min_freq"), "cpuinfo_min_freq")
	if err != nil {
		return policySnapshot{}, err
	}
	hardwareMax, err := readPositiveInt(filepath.Join(path, "cpuinfo_max_freq"), "cpuinfo_max_freq")
	if err != nil {
		return policySnapshot{}, err
	}
	governor := strings.TrimSpace(readFile(filepath.Join(path, "scaling_governor")))
	if governor == "" {
		return policySnapshot{}, errors.New("CPU governor is unavailable")
	}
	available := map[string]bool{}
	for _, value := range strings.Fields(readFile(filepath.Join(path, "scaling_available_governors"))) {
		available[value] = true
	}
	return policySnapshot{path: path, governor: governor, minimumKHz: minimum, maximumKHz: maximum, hardwareMin: hardwareMin, hardwareMax: hardwareMax, available: available}, nil
}

func readPositiveInt(path, label string) (int64, error) {
	value, err := strconv.ParseInt(strings.TrimSpace(readFile(path)), 10, 64)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("invalid %s for CPU policy", label)
	}
	return value, nil
}

func selectGovernor(profile string, snapshots []policySnapshot) (string, error) {
	candidates := []string{"schedutil", "ondemand", "conservative"}
	if profile == "turbo" {
		candidates = []string{"performance"}
	}
	for _, candidate := range candidates {
		availableEverywhere := true
		for _, snapshot := range snapshots {
			if !snapshot.available[candidate] {
				availableEverywhere = false
				break
			}
		}
		if availableEverywhere {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no safe governor is available for %s profile", profile)
}

func verifyApplied(snapshots []policySnapshot, profile, governor string) error {
	for _, snapshot := range snapshots {
		if strings.TrimSpace(readFile(filepath.Join(snapshot.path, "scaling_governor"))) != governor {
			return errors.New("CPU governor verification failed")
		}
		maximum, err := strconv.ParseInt(strings.TrimSpace(readFile(filepath.Join(snapshot.path, "scaling_max_freq"))), 10, 64)
		if err != nil {
			return errors.New("CPU frequency limit verification failed")
		}
		expected := snapshot.hardwareMax
		if profile == "eco" {
			expected = snapshot.hardwareMax * EcoMaximumPercent / 100
			if expected < snapshot.hardwareMin {
				expected = snapshot.hardwareMin
			}
		}
		if maximum != expected {
			return errors.New("CPU frequency limit verification failed")
		}
	}
	return nil
}

func (m *Manager) restore(snapshots []policySnapshot, noTurboPath string, noTurboOriginal int64, noTurboAvailable bool) error {
	var failures []string
	if noTurboAvailable {
		if err := m.writeInt(noTurboPath, noTurboOriginal); err != nil {
			failures = append(failures, err.Error())
		}
	}
	for _, snapshot := range snapshots {
		if err := m.writeInt(filepath.Join(snapshot.path, "scaling_min_freq"), snapshot.hardwareMin); err != nil {
			failures = append(failures, err.Error())
		}
		if err := m.writeInt(filepath.Join(snapshot.path, "scaling_max_freq"), snapshot.maximumKHz); err != nil {
			failures = append(failures, err.Error())
		}
		if snapshot.minimumKHz != snapshot.hardwareMin {
			if err := m.writeInt(filepath.Join(snapshot.path, "scaling_min_freq"), snapshot.minimumKHz); err != nil {
				failures = append(failures, err.Error())
			}
		}
		if err := m.writeString(filepath.Join(snapshot.path, "scaling_governor"), snapshot.governor); err != nil {
			failures = append(failures, err.Error())
		}
	}
	if len(failures) > 0 {
		return errors.New(strings.Join(failures, "; "))
	}
	return nil
}

func (m *Manager) writeInt(path string, value int64) error {
	return m.writeString(path, strconv.FormatInt(value, 10))
}
func (m *Manager) writeString(path, value string) error {
	writer := m.WriteFile
	if writer == nil {
		writer = os.WriteFile
	}
	if err := writer(path, []byte(value), 0o644); err != nil {
		return fmt.Errorf("write CPU power policy: %w", err)
	}
	return nil
}

func readFile(path string) string { content, _ := os.ReadFile(path); return string(content) }
func readOptionalInt(path string) (int64, bool) {
	value, err := strconv.ParseInt(strings.TrimSpace(readFile(path)), 10, 64)
	return value, err == nil
}

type Client struct {
	Enabled    bool
	SocketPath string
}

func (c Client) Available() bool {
	if !c.Enabled {
		return false
	}
	path := c.SocketPath
	if path == "" {
		path = DefaultSocketPath
	}
	info, err := os.Stat(path)
	return err == nil && info.Mode()&os.ModeSocket != 0
}

func (c Client) Apply(ctx context.Context, profile string) (Result, error) {
	if err := ValidateProfile(profile); err != nil {
		return Result{}, err
	}
	if !c.Enabled {
		return Result{}, ErrDisabled
	}
	path := c.SocketPath
	if path == "" {
		path = DefaultSocketPath
	}
	connection, err := (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, "unix", path)
	if err != nil {
		return Result{}, errors.New("power helper is unavailable")
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(5 * time.Second))
	if err := json.NewEncoder(connection).Encode(request{Profile: profile}); err != nil {
		return Result{}, errors.New("could not send power profile request")
	}
	var reply response
	decoder := json.NewDecoder(io.LimitReader(connection, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&reply); err != nil {
		return Result{}, errors.New("invalid response from power helper")
	}
	if reply.Error != "" {
		return Result{}, errors.New(reply.Error)
	}
	if reply.Result == nil {
		return Result{}, errors.New("power helper returned no result")
	}
	return *reply.Result, nil
}

func Serve(socketPath string, manager *Manager) error {
	if socketPath == "" {
		socketPath = DefaultSocketPath
	}
	if manager == nil {
		return errors.New("power manager is required")
	}
	if profile, ok := manager.persistedProfile(); ok {
		if _, err := manager.Apply(profile); err != nil {
			return fmt.Errorf("restore persisted power profile: %w", err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o750); err != nil {
		return err
	}
	if err := os.Remove(socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return err
	}
	defer listener.Close()
	defer os.Remove(socketPath)
	if err := os.Chmod(socketPath, 0o660); err != nil {
		return err
	}
	for {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			return acceptErr
		}
		handleConnection(connection, manager)
	}
}

func handleConnection(connection net.Conn, manager *Manager) {
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(5 * time.Second))
	decoder := json.NewDecoder(io.LimitReader(connection, 2048))
	decoder.DisallowUnknownFields()
	var input request
	if err := decoder.Decode(&input); err != nil {
		_ = json.NewEncoder(connection).Encode(response{Error: "invalid power helper request"})
		return
	}
	result, err := manager.Apply(input.Profile)
	if err != nil {
		message := "power profile could not be applied"
		if result.RollbackApplied {
			message += "; previous settings were restored"
		}
		_ = json.NewEncoder(connection).Encode(response{Error: message})
		return
	}
	_ = json.NewEncoder(connection).Encode(response{Result: &result})
}
