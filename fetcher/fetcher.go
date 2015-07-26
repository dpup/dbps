// Copyright 2015 Daniel Pupius

// Package fetcher provides an in-memory, read through cache, of image data
// backed by a downloader. No purging.

package fetcher

import (
	"expvar"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"path"
	"sync"
)

type Fetcher struct {
	folder     string
	downloader downloader

	cache     map[string]*cacheEntry
	cacheLock sync.Mutex
	cacheSize *expvar.Int
}

// Subset of the DropBox interface that satisfies our the usage.
type downloader interface {
	// Path, Revision, Offset
	Download(string, string, int) (io.ReadCloser, int64, error)
}

type cacheEntry struct {
	wg    sync.WaitGroup
	bytes []byte
	err   error
}

func New(folder string, d downloader) *Fetcher {
	return &Fetcher{
		folder:     folder,
		downloader: d,
		cache:      make(map[string]*cacheEntry),
		cacheSize:  expvar.NewInt(fmt.Sprintf("imageCacheSize (%s)", folder)),
	}
}

// Get returns the data for an image, falling back to the downloader if the
// image hasn't yet been loaded.
func (f *Fetcher) Get(filename string) ([]byte, error) {
	dbpath := path.Join(f.folder, filename)

	f.cacheLock.Lock()
	if entry, ok := f.cache[dbpath]; ok {
		f.cacheLock.Unlock()
		entry.wg.Wait()
		log.Printf("imgfetcher: cached fetch for %s", dbpath)
		return entry.bytes, entry.err
	}

	// TODO(dan): Throttle number of concurrent Gets to avoid DropBox rate limits.

	// Create the cache entry for future callers to wait on.
	entry := &cacheEntry{}
	entry.wg.Add(1)
	f.cache[dbpath] = entry
	f.cacheLock.Unlock()

	// Fetch the data.
	entry.bytes, entry.err = f.fetch(dbpath)
	entry.wg.Done()

	f.cacheLock.Lock()
	// We allow the error to be handled by current waiters, but don't persist it
	// for future callers.
	if entry.err != nil {
		delete(f.cache, dbpath)
	} else {
		f.cacheSize.Add(int64(len(entry.bytes)))
	}
	f.cacheLock.Unlock()

	return entry.bytes, entry.err
}

// Remove takes an entry out of the cache.
func (f *Fetcher) Remove(filename string) bool {
	f.cacheLock.Lock()
	defer f.cacheLock.Unlock()
	dbpath := path.Join(f.folder, filename)
	if entry, ok := f.cache[dbpath]; ok {
		f.cacheSize.Add(int64(-len(entry.bytes)))
		delete(f.cache, dbpath)
		return true
	}
	return false
}

func (f *Fetcher) fetch(dbpath string) ([]byte, error) {
	// TODO(dan): Add timeout, Download gets stuck.
	log.Printf("fetcher: fetching %s", dbpath)
	reader, _, err := f.downloader.Download(dbpath, "", 0)
	if err != nil {
		return []byte{}, err
	}
	return ioutil.ReadAll(reader)
}
