package system

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	historyRetention   = 24 * time.Hour
	historyMinInterval = 20 * time.Second
	historyMaxPoints   = 4_500
)

type ProcessorHistoryPoint struct {
	SocketID           string  `json:"socketId"`
	UtilizationPercent float64 `json:"utilizationPercent"`
}

type HistoryPoint struct {
	Timestamp              time.Time               `json:"timestamp"`
	CPUPercent             float64                 `json:"cpuPercent"`
	Processors             []ProcessorHistoryPoint `json:"processors"`
	MemoryPercent          float64                 `json:"memoryPercent"`
	SwapPercent            float64                 `json:"swapPercent"`
	DiskPercent            float64                 `json:"diskPercent"`
	MaximumTemperature     float64                 `json:"maximumTemperature"`
	ReceiveBytesPerSecond  float64                 `json:"receiveBytesPerSecond"`
	TransmitBytesPerSecond float64                 `json:"transmitBytesPerSecond"`
}

type HistoryStore struct {
	path        string
	mu          sync.Mutex
	initialized bool
	points      []HistoryPoint
	writes      int
}

func NewHistoryStore(path string) *HistoryStore {
	return &HistoryStore{path: path}
}

func HistoryPointFromMetrics(metrics Metrics) HistoryPoint {
	usage := UsageSummary(metrics)
	processors := make([]ProcessorHistoryPoint, 0, len(metrics.System.Processors))
	for _, processor := range metrics.System.Processors {
		processors = append(processors, ProcessorHistoryPoint{SocketID: processor.SocketID, UtilizationPercent: processor.UtilizationPercent})
	}
	return HistoryPoint{
		Timestamp: metrics.Timestamp, CPUPercent: metrics.CPUPercent, Processors: processors,
		MemoryPercent: usage["memoryPercent"], SwapPercent: usage["swapPercent"], DiskPercent: usage["diskPercent"],
		MaximumTemperature: usage["maxTemperature"], ReceiveBytesPerSecond: metrics.Network.ReceiveBytesPerSecond,
		TransmitBytesPerSecond: metrics.Network.TransmitBytesPerSecond,
	}
}

func (s *HistoryStore) Append(point HistoryPoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.initialize(); err != nil {
		return err
	}
	if point.Timestamp.IsZero() {
		return errors.New("history point timestamp is required")
	}
	if len(s.points) > 0 && point.Timestamp.Sub(s.points[len(s.points)-1].Timestamp) < historyMinInterval {
		return nil
	}
	s.points = append(s.points, point)
	s.writes++
	cutoff := point.Timestamp.Add(-historyRetention)
	s.points = retainedHistory(s.points, cutoff)
	if len(s.points) > historyMaxPoints {
		s.points = append([]HistoryPoint(nil), s.points[len(s.points)-historyMaxPoints:]...)
	}
	if s.writes == 1 || s.writes%120 == 0 {
		return s.rewrite()
	}
	return s.appendLine(point)
}

func (s *HistoryStore) ReadSince(since time.Time, maximum int) ([]HistoryPoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.initialize(); err != nil {
		return nil, err
	}
	points := retainedHistory(s.points, since)
	if maximum <= 0 || len(points) <= maximum {
		return append([]HistoryPoint(nil), points...), nil
	}
	return downsampleHistory(points, maximum), nil
}

func (s *HistoryStore) initialize() error {
	if s.initialized {
		return nil
	}
	s.initialized = true
	file, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open metric history: %w", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		var point HistoryPoint
		if json.Unmarshal(scanner.Bytes(), &point) == nil && !point.Timestamp.IsZero() {
			s.points = append(s.points, point)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read metric history: %w", err)
	}
	if len(s.points) > historyMaxPoints {
		s.points = append([]HistoryPoint(nil), s.points[len(s.points)-historyMaxPoints:]...)
	}
	return nil
}

func (s *HistoryStore) appendLine(point HistoryPoint) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create metric history directory: %w", err)
	}
	file, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open metric history for append: %w", err)
	}
	defer file.Close()
	if err := json.NewEncoder(file).Encode(point); err != nil {
		return fmt.Errorf("append metric history: %w", err)
	}
	return nil
}

func (s *HistoryStore) rewrite() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create metric history directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(s.path), ".history-*.jsonl")
	if err != nil {
		return fmt.Errorf("create metric history file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("protect metric history file: %w", err)
	}
	encoder := json.NewEncoder(temporary)
	for _, point := range s.points {
		if err := encoder.Encode(point); err != nil {
			temporary.Close()
			return fmt.Errorf("write metric history: %w", err)
		}
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync metric history: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close metric history: %w", err)
	}
	if err := os.Rename(temporaryPath, s.path); err != nil {
		return fmt.Errorf("replace metric history: %w", err)
	}
	return nil
}

func retainedHistory(points []HistoryPoint, since time.Time) []HistoryPoint {
	index := 0
	for index < len(points) && points[index].Timestamp.Before(since) {
		index++
	}
	return append([]HistoryPoint(nil), points[index:]...)
}

func downsampleHistory(points []HistoryPoint, maximum int) []HistoryPoint {
	if maximum <= 1 {
		return []HistoryPoint{points[len(points)-1]}
	}
	result := make([]HistoryPoint, 0, maximum)
	lastIndex := -1
	for index := 0; index < maximum; index++ {
		pointIndex := index * (len(points) - 1) / (maximum - 1)
		if pointIndex != lastIndex {
			result = append(result, points[pointIndex])
			lastIndex = pointIndex
		}
	}
	return result
}
