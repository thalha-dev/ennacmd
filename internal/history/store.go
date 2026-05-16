package history

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type Entry struct {
	Prompt     string    `json:"prompt"`
	Response   string    `json:"response"`
	Kind       string    `json:"kind"`
	Shell      string    `json:"shell"`
	Provider   string    `json:"provider"`
	Model      string    `json:"model"`
	Timestamp  time.Time `json:"timestamp"`
}

type Store struct {
	path  string
	limit int
	mu    sync.Mutex
}

func NewStore(path string, limit int) *Store {
	return &Store{path: path, limit: limit}
}

func (s *Store) Load(ctx context.Context) ([]Entry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, fmt.Errorf("read history: %w", err)
	}

	var entries []Entry
	if len(data) == 0 {
		return entries, nil
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("decode history: %w", err)
	}
	return entries, nil
}

func (s *Store) Append(ctx context.Context, entry Entry) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := s.Load(ctx)
	if err != nil {
		return err
	}

	entries = append(entries, entry)
	if s.limit > 0 && len(entries) > s.limit {
		entries = entries[len(entries)-s.limit:]
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("encode history: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(s.path, data, 0o600); err != nil {
		return fmt.Errorf("write history: %w", err)
	}

	return nil
}
