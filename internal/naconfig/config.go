// Package naconfig provides lazily refreshed Nacos configuration snapshots.
package naconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bihua-university/alisten/internal/logx"
	"github.com/bihua-university/alisten/internal/nacos"
	pb "github.com/bihua-university/alisten/internal/proto/nacos"
)

const defaultRefreshTTL = 5 * time.Minute

var refreshTTL = defaultRefreshTTL

type snapshot[T any] struct {
	value     T
	expiresAt time.Time
}

// Config holds an immutable configuration snapshot.
type Config[T any] struct {
	dataID string
	decode func([]byte) (T, error)
	value  atomic.Pointer[snapshot[T]]

	refreshMu sync.Mutex
}

// Json declares a JSON configuration without performing I/O.
func Json[T any](dataID string) *Config[T] {
	return &Config[T]{
		dataID: dataID,
		decode: func(data []byte) (T, error) {
			var value T
			err := json.Unmarshal(data, &value)
			return value, err
		},
	}
}

// Get returns a fresh snapshot, synchronously refreshing an expired one on demand.
// It panics if the initial load fails and returns the expired snapshot if a later refresh fails.
func (c *Config[T]) Get() T {
	if current := c.value.Load(); fresh(current) {
		return current.value
	}
	c.refreshMu.Lock()
	defer c.refreshMu.Unlock()
	current := c.value.Load()
	if fresh(current) {
		return current.value
	}
	if err := c.refreshLocked(context.Background()); err != nil {
		if current == nil {
			panic(err)
		}
		logx.Errorf("refresh Nacos config %s: %v; using expired snapshot", c.dataID, err)
		return current.value
	}
	return c.value.Load().value
}

// Refresh fetches and publishes a new snapshot, retaining the old one on failure.
func (c *Config[T]) Refresh(ctx context.Context) error {
	c.refreshMu.Lock()
	defer c.refreshMu.Unlock()
	return c.refreshLocked(ctx)
}

// Update replaces the local snapshot without fetching or publishing to Nacos.
func (c *Config[T]) Update(value T) {
	c.value.Store(&snapshot[T]{value: value, expiresAt: time.Now().Add(refreshTTL)})
}

func fresh[T any](current *snapshot[T]) bool {
	return current != nil && time.Now().Before(current.expiresAt)
}

func (c *Config[T]) refreshLocked(ctx context.Context) error {
	reply, err := nacos.GetConfig(ctx, &pb.ConfigRequest{DataId: c.dataID})
	if err != nil {
		return fmt.Errorf("naconfig load %s: %w", c.dataID, err)
	}
	value, err := c.decode([]byte(reply.Content))
	if err != nil {
		return fmt.Errorf("naconfig decode %s: %w", c.dataID, err)
	}
	c.value.Store(&snapshot[T]{value: value, expiresAt: time.Now().Add(refreshTTL)})
	return nil
}
