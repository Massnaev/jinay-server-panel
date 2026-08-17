package audit

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

type Entry struct {
	Timestamp time.Time `json:"timestamp"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Target    string    `json:"target,omitempty"`
	Result    string    `json:"result"`
	RemoteIP  string    `json:"remoteIp,omitempty"`
	Detail    string    `json:"detail,omitempty"`
}

type Log struct {
	path string
	mu   sync.Mutex
}

func New(path string) *Log {
	return &Log{path: path}
}

func (l *Log) Append(entry Entry) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := os.Chmod(l.path, 0o600); err != nil {
		return err
	}
	return json.NewEncoder(file).Encode(entry)
}

func (l *Log) Tail(limit int) ([]Entry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	file, err := os.Open(l.path)
	if errors.Is(err, os.ErrNotExist) {
		return []Entry{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	entries := []Entry{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry Entry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return nil, fmt.Errorf("decode audit log: %w", err)
		}
		entries = append(entries, entry)
		if len(entries) > limit {
			entries = entries[1:]
		}
	}
	return entries, scanner.Err()
}
