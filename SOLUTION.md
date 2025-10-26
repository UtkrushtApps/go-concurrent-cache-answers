# Solution Steps

1. Create a new package directory named 'cache' for the cache implementation files.

2. In cache/cache.go, define a struct 'Cache' containing a sync.RWMutex (as mu), a map for data, a sync.WaitGroup, context and cancel function for cleanup coordination, and a field for the cleanup interval.

3. Design a struct 'entry' that holds the value (interface{}) and the expire time (time.Time).

4. Implement the constructor New(), which initializes the cache, context, and launches a background goroutine for expired-entry cleanup. The goroutine uses the provided interval, listens for context cancellation, and signals via WaitGroup when finished.

5. Implement Set(key, value, ttl), using a write lock. If ttl  0, set expiresAt to zero time (never expires).

6. Implement Get(key) using read lock for fast reads. If the entry is missing or expired, return (nil, false). If expired, remove the key using write lock (upgrade pattern).

7. Implement Delete(key) using write lock to safely remove a key from the map.

8. Implement Stop(), which uses cancellation, waits for cleanup goroutine to exit via WaitGroup, and allows clean shutdown.

9. Implement cleanupExpiredEntries() as background goroutine: every cleanerInterval, scans the map under write lock and deletes all expired entries.

10. Create cache/cache_test.go and write tests covering: basic Get/Set/Delete, TTL expiry, non-TTL'ed keys surviving expiry cycles, race safety with 10 concurrent readers and 2 concurrent writers, correct expiry in presence of race, proper background cleanup and goroutine shutdown, and confirmation that Get also correctly removes expired entries from the map.

11. Run go test -race ./cache to verify no race conditions, correct cleanup, and that all tests pass.

