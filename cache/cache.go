// Copyright 2015 Daniel Pupius

// Package cache provides a generic in-memory, read through cache, of []byte.
// No purging or expiration supported..

package cache

import (
	"expvar"
	"fmt"
	"sync"
)

type FetchFn func(string) ([]byte, error)

type Cache struct {
	fetchFn   FetchFn
	cache     map[string]*cacheEntry
	cacheLock sync.Mutex
	cacheSize *expvar.Int
}

type cacheEntry struct {
	wg    sync.WaitGroup
	bytes []byte
	err   error
}

func New(name string, fetchFn FetchFn) *Cache {
	return &Cache{
		fetchFn:   fetchFn,
		cache:     make(map[string]*cacheEntry),
		cacheSize: expvar.NewInt(fmt.Sprintf("cacheSize (%s)", name)),
	}
}

// Get returns the data for a key, falling back to the FetchFn if the
// data hasn't yet been loaded.
func (c *Cache) Get(key string) ([]byte, error) {

	c.cacheLock.Lock()
	if entry, ok := c.cache[key]; ok {
		c.cacheLock.Unlock()
		entry.wg.Wait()
		return entry.bytes, entry.err
	}

	// TODO(dan): Throttle number of concurrent Gets to avoid DropBox rate limits.

	// Create the cache entry for future callers to wait on.
	entry := &cacheEntry{}
	entry.wg.Add(1)
	c.cache[key] = entry
	c.cacheLock.Unlock()

	// Fetch the data.
	entry.bytes, entry.err = c.fetchFn(key)
	entry.wg.Done()

	c.cacheLock.Lock()

	// We allow the error to be handled by current waiters, but don't persist it
	// for future callers.
	if entry.err != nil {
		delete(c.cache, key)
	} else {
		c.cacheSize.Add(int64(len(entry.bytes)))
	}
	c.cacheLock.Unlock()

	return entry.bytes, entry.err
}

// Remove takes an entry out of the cache.
func (c *Cache) Remove(key string) bool {
	c.cacheLock.Lock()
	defer c.cacheLock.Unlock()
	if entry, ok := c.cache[key]; ok {
		c.cacheSize.Add(int64(-len(entry.bytes)))
		delete(c.cache, key)
		return true
	}
	return false
}
