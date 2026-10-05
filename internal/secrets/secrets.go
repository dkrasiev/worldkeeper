// Package secrets keeps passwords in the OS credential store:
// Keychain on macOS, Credential Manager on Windows, Secret Service on Linux.
package secrets

import (
	"errors"
	"sync"

	"github.com/zalando/go-keyring"
)

const service = "worldkeeper"

var ErrNotFound = errors.New("secret not found")

type Store interface {
	Get(key string) (string, error)
	Set(key, value string) error
}

// Keyring uses the OS credential store.
type Keyring struct{}

func (Keyring) Get(key string) (string, error) {
	v, err := keyring.Get(service, key)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	return v, err
}

func (Keyring) Set(key, value string) error {
	return keyring.Set(service, key, value)
}

// Memory is an in-process store for tests.
type Memory struct {
	mu sync.Mutex
	m  map[string]string
}

func (s *Memory) Get(key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[key]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

func (s *Memory) Set(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m == nil {
		s.m = map[string]string{}
	}
	s.m[key] = value
	return nil
}
