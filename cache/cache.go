package cache

import (
	"context"
	"sync"
	"time"
)

// entry wraps a value with its expiration time.
type entry struct {
	value      interface{}
	expiresAt  time.Time // If zero value, never expires
}

// Cache is a concurrent, in-memory key-value cache with per-entry TTL and background cleanup.
type Cache struct {
	mu      sync.RWMutex
	data    map[string]entry
	wg      sync.WaitGroup
	cancel  context.CancelFunc
	ctx     context.Context
	cleanerInterval time.Duration
}

// New creates a new Cache. Starts the background cleanup goroutine with given cleanup interval.
func New(cleanerInterval time.Duration) *Cache {
	ctx, cancel := context.WithCancel(context.Background())
	c := &Cache{
		data:   make(map[string]entry),
		ctx:    ctx,
		cancel: cancel,
		cleanerInterval: cleanerInterval,
	}
	c.wg.Add(1)
	go c.cleanupExpiredEntries()
	return c
}

// Set inserts or updates a value in the cache with optional TTL.
// If ttl <= 0, never expires.
func (c *Cache) Set(key string, value interface{}, ttl time.Duration) {
	expiresAt := time.Time{}
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = entry{
		value:     value,
		expiresAt: expiresAt,
	}
}

// Get retrieves a value. Returns (value, true) if found and not expired, else (nil, false)
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	entry, ok := c.data[key]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		// Key expired, remove it
		c.mu.Lock()
		// Double-check expiry and existence
		entry2, stillOk := c.data[key]
		if stillOk && entry2.expiresAt.Equal(entry.expiresAt) {
			delete(c.data, key)
		}
		c.mu.Unlock()
		return nil, false
	}
	return entry.value, true
}

// Delete removes a key from the cache
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
}

// Stop stops the background cleanup goroutine and waits for completion.
func (c *Cache) Stop() {
	c.cancel()
	c.wg.Wait()
}

// cleanupExpiredEntries periodically scans for expired entries and deletes them.
func (c *Cache) cleanupExpiredEntries() {
	ticker := time.NewTicker(c.cleanerInterval)
	defer func() {
		ticker.Stop()
		c.wg.Done()
	}()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			c.mu.Lock()
			for k, e := range c.data {
				if !e.expiresAt.IsZero() && now.After(e.expiresAt) {
					delete(c.data, k)
				}
			}
			c.mu.Unlock()
		}
	}
}
