package system

import (
	"context"
	"errors"
	"time"
)

type Finding struct {
	ID             string    `json:"id"`
	Severity       string    `json:"severity"`
	Title          string    `json:"title"`
	Detail         string    `json:"detail"`
	Recommendation string    `json:"recommendation"`
	DetectedAt     time.Time `json:"detectedAt"`
}

func Diagnose(ctx context.Context, docker Docker) ([]Finding, error) {
	metrics, err := ReadMetrics()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	usage := UsageSummary(metrics)
	findings := []Finding{}
	if usage["cpuPercent"] >= 90 {
		findings = append(findings, Finding{ID: "cpu-saturation", Severity: "critical", Title: "CPU is saturated", Detail: "Processor use is above 90%.", Recommendation: "Inspect container and process usage before enabling turbo mode.", DetectedAt: now})
	}
	if usage["memoryPercent"] >= 90 {
		findings = append(findings, Finding{ID: "memory-pressure", Severity: "critical", Title: "Memory pressure", Detail: "Available memory is below 10%.", Recommendation: "Find the largest consumers and verify swap and OOM events.", DetectedAt: now})
	}
	if usage["diskPercent"] >= 85 {
		findings = append(findings, Finding{ID: "disk-capacity", Severity: "warning", Title: "Disk space is running low", Detail: "Root filesystem usage is above 85%.", Recommendation: "Review container images, logs and backups before removing data.", DetectedAt: now})
	}
	if usage["maxTemperature"] >= 85 {
		findings = append(findings, Finding{ID: "thermal-limit", Severity: "critical", Title: "High component temperature", Detail: "A reported sensor is at or above 85°C.", Recommendation: "Reduce load and inspect cooling. Do not override fan safety limits.", DetectedAt: now})
	}
	if _, err := docker.List(ctx); err != nil {
		severity := "warning"
		detail := err.Error()
		if errors.Is(err, ErrDockerUnavailable) {
			severity = "info"
			detail = "Docker is not installed or is not visible to the agent."
		}
		findings = append(findings, Finding{ID: "docker-unavailable", Severity: severity, Title: "Docker inventory unavailable", Detail: detail, Recommendation: "Verify Docker installation and the least-privilege agent permission model.", DetectedAt: now})
	}
	if len(findings) == 0 {
		findings = append(findings, Finding{ID: "healthy", Severity: "ok", Title: "No active findings", Detail: "Current thresholds are within the MVP safe range.", Recommendation: "Continue monitoring trends and configure backups.", DetectedAt: now})
	}
	return findings, nil
}
