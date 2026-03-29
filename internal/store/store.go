package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// JsonStore is a generic, thread-safe JSON file store with atomic writes.
type JsonStore[T any] struct {
	mu   sync.RWMutex
	path string
	data T
}

// New loads the store from path. If the file does not exist, the store starts empty.
func New[T any](path string) (*JsonStore[T], error) {
	s := &JsonStore[T]{path: path}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(&s.data); err != nil {
		return nil, err
	}
	return s, nil
}

// Get returns a copy of the current data.
func (s *JsonStore[T]) Get() T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data
}

// Set replaces the data and persists it atomically.
func (s *JsonStore[T]) Set(data T) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = data
	return s.flush()
}

// Update applies fn to the data in place, then persists atomically.
func (s *JsonStore[T]) Update(fn func(*T)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.data)
	return s.flush()
}

// flush writes data to a temp file then renames it (atomic on POSIX).
func (s *JsonStore[T]) flush() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s.data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, s.path)
}
