package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"raceguard/internal/config"
)

const maxHistoryEntries = 100

// Store manages thread-safe local persistence of test run history (no CGO required).
type Store struct {
	mu   sync.Mutex
	path string
}

// New creates a local JSON-backed history store at the given file path.
func New(path string) *Store {
	return &Store{path: path}
}

// Append saves a new RunResult at the top of history, capping total entries.
func (s *Store) Append(res config.RunResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	history, _ := s.loadUnlocked()
	history = append([]config.RunResult{res}, history...)
	if len(history) > maxHistoryEntries {
		history = history[:maxHistoryEntries]
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0o600)
}

// List returns stored RunResults ordered newest first.
func (s *Store) List() ([]config.RunResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadUnlocked()
}

// Clear removes all saved run history.
func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *Store) loadUnlocked() ([]config.RunResult, error) {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []config.RunResult{}, nil
		}
		return nil, err
	}
	var history []config.RunResult
	if err := json.Unmarshal(raw, &history); err != nil {
		return []config.RunResult{}, err
	}
	return history, nil
}
