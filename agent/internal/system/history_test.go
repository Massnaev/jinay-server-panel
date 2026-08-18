package system

import (
	"path/filepath"
	"testing"
	"time"
)

func TestHistoryStorePersistsBoundsAndDownsamples(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	store := NewHistoryStore(path)
	start := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	for index := 0; index < 6; index++ {
		point := HistoryPoint{Timestamp: start.Add(time.Duration(index) * time.Minute), CPUPercent: float64(index)}
		if err := store.Append(point); err != nil {
			t.Fatal(err)
		}
	}

	reopened := NewHistoryStore(path)
	points, err := reopened.ReadSince(start.Add(2*time.Minute), 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 3 {
		t.Fatalf("expected three downsampled points, got %d", len(points))
	}
	if points[0].CPUPercent != 2 || points[len(points)-1].CPUPercent != 5 {
		t.Fatalf("unexpected downsample endpoints: %+v", points)
	}
}

func TestHistoryStoreSkipsDenseAndExpiredPoints(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	store := NewHistoryStore(path)
	now := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	for _, point := range []HistoryPoint{
		{Timestamp: now.Add(-25 * time.Hour), CPUPercent: 10},
		{Timestamp: now, CPUPercent: 20},
		{Timestamp: now.Add(5 * time.Second), CPUPercent: 30},
		{Timestamp: now.Add(time.Minute), CPUPercent: 40},
	} {
		if err := store.Append(point); err != nil {
			t.Fatal(err)
		}
	}
	points, err := store.ReadSince(now.Add(-24*time.Hour), 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 2 || points[0].CPUPercent != 20 || points[1].CPUPercent != 40 {
		t.Fatalf("unexpected retained history: %+v", points)
	}
}

func TestHistoryPointFromMetrics(t *testing.T) {
	metrics := Metrics{
		Timestamp: time.Now().UTC(), CPUPercent: 35,
		System:           SystemInfo{Processors: []ProcessorInfo{{SocketID: "0", UtilizationPercent: 20}, {SocketID: "1", UtilizationPercent: 50}}},
		MemoryTotalBytes: 100, MemoryUsedBytes: 60, SwapTotalBytes: 100, SwapUsedBytes: 10,
		Temperatures: []Temperature{{Celsius: 61}}, Network: Network{ReceiveBytesPerSecond: 1024, TransmitBytesPerSecond: 512},
	}
	point := HistoryPointFromMetrics(metrics)
	if len(point.Processors) != 2 || point.Processors[1].UtilizationPercent != 50 || point.MemoryPercent != 60 || point.MaximumTemperature != 61 {
		t.Fatalf("unexpected history point: %+v", point)
	}
}
