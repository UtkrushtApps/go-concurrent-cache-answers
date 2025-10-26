package cache

import (
	"context"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCacheBasicOperations(t *testing.T) {
	c := New(20 * time.Millisecond)
	defer c.Stop()

	key := "foo"
	val := "bar"
	c.Set(key, val, 1*time.Second)
	v, ok := c.Get(key)
	if !ok || v != val {
		t.Fatalf("expected %v, got %v, ok=%v", val, v, ok)
	}

	c.Delete(key)
	v, ok = c.Get(key)
	if ok {
		t.Fatalf("expected key to be deleted")
	}
}

func TestCacheTTLExpiry(t *testing.T) {
	c := New(10 * time.Millisecond)
	defer c.Stop()

	key := "exp"
	val := 123
	c.Set(key, val, 30*time.Millisecond)
	v, ok := c.Get(key)
	if !ok || v != val {
		t.Fatalf("want %v, got %v ok=%v", val, v, ok)
	}

	time.Sleep(50 * time.Millisecond)
	v, ok = c.Get(key)
	if ok {
		t.Fatalf("expected key to be expired")
	}
}

func TestCacheNoTTLPersists(t *testing.T) {
	c := New(10 * time.Millisecond)
	defer c.Stop()
	key := "bazz"
	val := 999
	c.Set(key, val, 0)
	time.Sleep(40 * time.Millisecond)
	v, ok := c.Get(key)
	if !ok || v != val {
		t.Fatalf("expected key to persist, got %v, ok=%v", v, ok)
	}
}

func TestConcurrentReadersWriters(t *testing.T) {
	c := New(20 * time.Millisecond)
	defer c.Stop()

	nReads := 10
	nWrites := 2
	var start sync.WaitGroup
	start.Add(1)

	var done sync.WaitGroup
	keyCount := 50

	var writeOps int64
	var readHits int64

	// Start writers
	for i := 0; i < nWrites; i++ {
		done.Add(1)
		go func(id int) {
			defer done.Done()
			start.Wait()
			rand.Seed(time.Now().UnixNano() + int64(id))
			for w := 0; w < 300; w++ {
				k := rand.Intn(keyCount)
				c.Set("k"+string(rune(k)), id*1000+w, 150*time.Millisecond)
				atomic.AddInt64(&writeOps, 1)
				// sleep short
				time.Sleep(time.Duration(rand.Intn(2)) * time.Millisecond)
			}
		}(i)
	}

	// Start readers
	for i := 0; i < nReads; i++ {
		done.Add(1)
		go func(id int) {
			defer done.Done()
			start.Wait()
			rand.Seed(time.Now().UnixNano() + int64(id*170))
			for r := 0; r < 1000; r++ {
				k := rand.Intn(keyCount)
				v, ok := c.Get("k" + string(rune(k)))
				_ = v
				if ok {
					atomic.AddInt64(&readHits, 1)
				}
				// sleep short (high read pressure)
				runtime.Gosched()
			}
		}(i)
	}

	start.Done() // start readers/writers
	done.Wait()

	// Some keys should have been expired after some time
	surviving := 0
	for i := 0; i < keyCount; i++ {
		_, ok := c.Get("k" + string(rune(i)))
		if ok {
			surviving++
		}
	}
	if surviving == keyCount {
		t.Fatalf("expected keys to expire, but all survived")
	}
}

func TestBackgroundCleanupShutdown(t *testing.T) {
	startG := runtime.NumGoroutine()
	c := New(10 * time.Millisecond)
	c.Set("x", 11, 5*time.Millisecond)
	c.Set("y", 22, 5*time.Millisecond)
	c.Set("z", 33, 5*time.Millisecond)
	time.Sleep(35 * time.Millisecond)
	c.Stop()
	// Allow Go scheduler to release any goroutine
	time.Sleep(10 * time.Millisecond)
	endG := runtime.NumGoroutine()
	if endG - startG > 2 { // Give buffer for other goroutines
		t.Fatalf("possible goroutine leak: delta=%d", endG-startG)
	}
	// All expired should be gone
	if _, ok := c.Get("x"); ok {
		t.Fatalf("expired key x still present")
	}
}

func TestGetRemovesExpiredKey(t *testing.T) {
	c := New(20 * time.Millisecond)
	defer c.Stop()
	c.Set("foo", "bar", 10*time.Millisecond)
	time.Sleep(35 * time.Millisecond)
	// Should be removed on Get
	if val, ok := c.Get("foo"); ok || val != nil {
		t.Fatalf("Get should remove and return nil/false for expired entry")
	}
}
