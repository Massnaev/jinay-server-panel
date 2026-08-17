package system

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var ErrDockerUnavailable = errors.New("docker is unavailable")
var containerIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)

type Container struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Image      string `json:"image"`
	Status     string `json:"status"`
	State      string `json:"state"`
	Ports      string `json:"ports"`
	RunningFor string `json:"runningFor"`
}

type dockerRow struct {
	ID         string `json:"ID"`
	Names      string `json:"Names"`
	Image      string `json:"Image"`
	Status     string `json:"Status"`
	State      string `json:"State"`
	Ports      string `json:"Ports"`
	RunningFor string `json:"RunningFor"`
}

type Docker struct {
	ActionsEnabled bool
}

func (d Docker) List(ctx context.Context) ([]Container, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "docker", "ps", "-a", "--format", "{{json .}}")
	output, err := command.Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, ErrDockerUnavailable
		}
		return nil, fmt.Errorf("list Docker containers: %w", err)
	}
	containers := []Container{}
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		var row dockerRow
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			return nil, fmt.Errorf("decode Docker row: %w", err)
		}
		containers = append(containers, Container{
			ID: row.ID, Name: row.Names, Image: row.Image, Status: row.Status,
			State: row.State, Ports: row.Ports, RunningFor: row.RunningFor,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read Docker output: %w", err)
	}
	return containers, nil
}

func (d Docker) Action(ctx context.Context, id, action string) error {
	if !d.ActionsEnabled {
		return errors.New("Docker actions are disabled by configuration")
	}
	if !containerIDPattern.MatchString(id) {
		return errors.New("invalid container identifier")
	}
	action = strings.ToLower(action)
	if action != "start" && action != "stop" && action != "restart" {
		return errors.New("unsupported Docker action")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "docker", action, id)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("docker %s failed: %s", action, strings.TrimSpace(string(output)))
	}
	return nil
}
