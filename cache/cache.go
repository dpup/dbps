// Copyright 2015 Daniel Pupius

// Package cache provides a generic in-memory, read through cache, of []byte.
// Cache keys are structs that can provide detailed parameters for the requested
// resource. CacheKeys declare what other keys they depend on, which allows for
// removal of down-stream entries.

package cache

import (
	"expvar"
	"fmt"
	"reflect"
	"sync"
)

var (
	byteArrayType = reflect.ValueOf([]byte{}).Type()
	errorType     = reflect.TypeOf((*error)(nil)).Elem()
)

type cacheEntry struct {
	wg    sync.WaitGroup
	bytes []byte
	err   error
}

type Cache struct {
	fetchers  map[reflect.Type]reflect.Value
	cache     map[CacheKey]*cacheEntry
	cacheLock sync.Mutex
	cacheSize *expvar.Int
}

func New(name string) *Cache {
	return &Cache{
		fetchers:  make(map[reflect.Type]reflect.Value),
		cache:     make(map[CacheKey]*cacheEntry),
		cacheSize: expvar.NewInt(fmt.Sprintf("cacheSize (%s)", name)),
	}
}

func (c *Cache) RegisterFetcher(fn interface{}) {
	v := reflect.ValueOf(fn)
	t := v.Type()
	if t.Kind() != reflect.Func {
		panic(fmt.Sprintf("cache: Fetcher must be a function, got [%v]", t))
	}
	if t.NumIn() != 1 {
		panic(fmt.Sprintf("cache: Fetcher must be function with one arg, has %d [%v]", t.NumIn(), t))
	}
	if t.NumOut() != 2 || t.Out(0) != byteArrayType || t.Out(1) != errorType {
		panic(fmt.Sprintf("cache: Fetcher must be function that returns ([]byte, error), has %d [%v]", t.NumOut(), t))
	}

	c.cacheLock.Lock()
	defer c.cacheLock.Unlock()

	// Map the argument type to the listener.
	arg := t.In(0)
	c.fetchers[arg] = v
}

// Get returns the data for a key, falling back to a fetcher function if the
// data hasn't yet been loaded.
func (c *Cache) Get(key CacheKey) ([]byte, error) {
	c.cacheLock.Lock()
	if entry, ok := c.cache[key]; ok {
		c.cacheLock.Unlock()
		entry.wg.Wait()
		return entry.bytes, entry.err
	}

	// Create the cache entry for future callers to wait on.
	entry := &cacheEntry{}
	entry.wg.Add(1)
	c.cache[key] = entry
	c.cacheLock.Unlock()

	// Use reflection to look up the right fetcher, then request the data.
	v := reflect.ValueOf(key)
	t := v.Type()
	if fetcher, ok := c.fetchers[t]; ok {
		values := fetcher.Call([]reflect.Value{v})
		// We've already verified types should be correct.
		entry.bytes = values[0].Bytes()
		if values[1].Interface() != nil {
			entry.err = values[1].Interface().(error)
		}
	} else {
		panic(fmt.Sprintf("cache: No fetcher function for type [%v]", t))
	}

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
func (c *Cache) Remove(key CacheKey) bool {
	c.cacheLock.Lock()
	defer c.cacheLock.Unlock()
	if entry, ok := c.cache[key]; ok {
		c.cacheSize.Add(int64(-len(entry.bytes)))
		delete(c.cache, key)
		return true
	}
	return false
}
