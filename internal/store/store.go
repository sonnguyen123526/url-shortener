// Package store persists short-code -> URL mappings.
package store

import (
	"context"
	"errors"
	"sync"

	"github.com/redis/go-redis/v9"
)

// ErrNotFound is returned when a short code does not exist.
var ErrNotFound = errors.New("not found")

// Store is the storage abstraction used by the HTTP handlers.
// Having an interface lets us swap Redis for an in-memory store in tests.
type Store interface {
	// Save stores url under code. It returns false if code is already taken.
	Save(ctx context.Context, code, url string) (bool, error)
	// Get returns the URL for code, or ErrNotFound.
	Get(ctx context.Context, code string) (string, error)
	// Ping reports whether the backing storage is reachable.
	Ping(ctx context.Context) error
}

// ---- Redis implementation ----

const keyPrefix = "url:"

type Redis struct {
	client *redis.Client
}

func NewRedis(client *redis.Client) *Redis {
	return &Redis{client: client}
}

func (r *Redis) Save(ctx context.Context, code, url string) (bool, error) {
	// SETNX only writes if the key does not exist, so codes are never overwritten.
	return r.client.SetNX(ctx, keyPrefix+code, url, 0).Result()
}

func (r *Redis) Get(ctx context.Context, code string) (string, error) {
	url, err := r.client.Get(ctx, keyPrefix+code).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrNotFound
	}
	return url, err
}

func (r *Redis) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// ---- In-memory implementation (for tests and local hacking) ----

type Memory struct {
	mu   sync.RWMutex
	data map[string]string
}

func NewMemory() *Memory {
	return &Memory{data: make(map[string]string)}
}

func (m *Memory) Save(_ context.Context, code, url string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.data[code]; exists {
		return false, nil
	}
	m.data[code] = url
	return true, nil
}

func (m *Memory) Get(_ context.Context, code string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	url, ok := m.data[code]
	if !ok {
		return "", ErrNotFound
	}
	return url, nil
}

func (m *Memory) Ping(context.Context) error { return nil }
