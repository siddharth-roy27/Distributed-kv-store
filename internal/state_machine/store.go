package state_machine

import (
	"fmt"
	"strings"
	"sync"
)

// Store is a simple in-memory KV store used as the Raft state machine.
type Store struct {
	mu sync.RWMutex
	m  map[string]string
}

func NewStore() *Store {
	return &Store{m: make(map[string]string)}
}

// Apply applies a string command to the state machine.
// Supported command format: "set:<key>:<value>".
func (s *Store) Apply(command string) error {
	parts := strings.SplitN(command, ":", 3)
	if len(parts) < 3 {
		return fmt.Errorf("invalid command: %s", command)
	}
	if parts[0] != "set" {
		return fmt.Errorf("unsupported command: %s", parts[0])
	}
	key := parts[1]
	value := parts[2]
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = value
	return nil
}

// Get returns the value for a key and a bool indicating if it existed.
func (s *Store) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.m[key]
	return v, ok
}
