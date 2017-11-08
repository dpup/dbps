// Copyright 2015 Daniel Pupius

package dbps

import (
	"bytes"
	"errors"
	"expvar"
	"fmt"
	"io/ioutil"
	"log"
	"path"
	"sort"
	"sync"
	"time"

	"github.com/dpup/dbps/internal/dropbox"
	"github.com/dpup/dbps/internal/goexif/exif"
	"github.com/dpup/rcache"
)

// Album queries dropbox and keeps a list of photos in date order.
type Album struct {
	folder  string
	dropbox *dropbox.Client
	cache   rcache.Cache

	photoList photoList
	photoMap  map[string]Photo
	loading   bool
	mu        sync.RWMutex
}

// NewAlbum returns a new Album
func NewAlbum(folder string, dropbox *dropbox.Client) *Album {
	a := &Album{folder: folder, dropbox: dropbox, cache: rcache.New(folder)}
	a.cache.RegisterFetcher(a.fetchOriginal)
	a.cache.RegisterFetcher(a.fetchThumbnail)

	expvar.Publish(fmt.Sprintf("photos (%s)", folder), expvar.Func(func() interface{} {
		return a.photoMap
	}))

	return a
}

// Monitor starts a go routine which calls Load() every interval to pick up new
// changes
func (a *Album) Monitor(interval time.Duration) {
	c := interval
	go func() {
		for {
			time.Sleep(c)
			err := a.Load()
			if err != nil {
				log.Printf("album: failed to refresh after %s: %s", c, err)
				c = c * 2
			} else {
				c = interval
			}
		}
	}()
}

// Load fetches metadata about the photos in a folder. If the folder hasn't
// changed since Load was last called then no work wil be done.
func (a *Album) Load() error {
	a.mu.Lock()
	if a.loading {
		a.mu.Unlock()
		return errors.New("album: load already in progress")
	}
	a.loading = true
	defer func() { a.loading = false }()
	a.mu.Unlock()

	log.Println("album: loading image metadata")

	f, err := a.dropbox.Files.ListFolder(&dropbox.ListFolderInput{
		Path:             a.folder,
		Limit:            2000,
		IncludeMediaInfo: true,
	})
	if err != nil {
		return fmt.Errorf("album: failed to list files: %s", err)
	}

	files := f.Entries

	var wg sync.WaitGroup
	photos := make(photoList, len(files))

	c := 0
	for i, e := range files {
		name := path.Base(e.PathLower)

		// If no entry exists, or the entry is stale, then load the photo to get its
		// exif data. Loads are done in parallel.
		if old, ok := a.photoMap[name]; !ok || old.Hash != e.ContentHash {
			photos[i] = Photo{
				Filename:        name,
				Size:            int(e.Size),
				Hash:            e.ContentHash,
				DropboxModified: e.ServerModified,
				ExifCreated:     e.ClientModified, // Default to the last modified time.
			}

			c++
			wg.Add(1)
			a.cache.Invalidate(originalCacheKey{name}, true)
			go a.loadExifInfo(&photos[i], &wg)

		} else {
			photos[i] = old
		}
	}
	if c > 0 {
		log.Printf("album: waiting for new images to load")
	} else {
		log.Printf("album: no new images")
	}
	wg.Wait()
	sort.Sort(photos)

	// TODO(dan): Currently we are not clearing the cache of deleted images, for
	// the existing usecase that is a rare scenario. Can easily be added by
	// asking for deleted items and checking entry.IsDeleted

	a.mu.Lock()
	a.photoList = photos
	a.photoMap = make(map[string]Photo)
	for _, p := range photos {
		a.photoMap[p.Filename] = p
	}
	a.mu.Unlock()

	log.Println("album: metadata load complete")

	return nil
}

// FirstPhoto returns the ... first photo.
func (a *Album) FirstPhoto() Photo {
	return a.photoList[0]
}

// Photo returns the metadata for a photo and the image data, or an error if it doesn't exist.
func (a *Album) Photo(name string) (Photo, []byte, error) {
	if photo, ok := a.photoMap[name]; ok {
		data, err := a.cache.Get(originalCacheKey{name})
		return photo, data, err
	}
	return Photo{}, nil, fmt.Errorf("album: no photo with name: %s", name)
}

// Thumbnail returns the metadata for a photo and a thumbnail, or an error if it doesn't exist.
func (a *Album) Thumbnail(name string, width, height uint) (Photo, []byte, error) {
	if photo, ok := a.photoMap[name]; ok {
		data, err := a.cache.Get(thumbCacheKey{name, width, height})
		return photo, data, err
	}
	return Photo{}, nil, fmt.Errorf("album: no photo with name: %s", name)
}

// Photos returns a copy of the PhotoList.
func (a *Album) Photos() []Photo {
	a.mu.RLock()
	defer a.mu.RUnlock()
	c := make(photoList, len(a.photoList))
	copy(c, a.photoList)
	return c
}

func (a *Album) loadExifInfo(p *Photo, wg *sync.WaitGroup) {
	defer func() { wg.Done() }()

	data, err := a.cache.Get(originalCacheKey{p.Filename})
	if err != nil {
		log.Printf("album: error renewing cache for %s: %s", p, err)
		return
	}

	x, err := exif.Decode(bytes.NewReader(data))
	if err != nil {
		log.Printf("album: error reading exif for %s: %s", p, err)
		return
	}

	t, err := x.DateTime()
	if err != nil {
		log.Printf("album: error reading exif datetime for %s: %s", p, err)
		return
	}

	p.ExifCreated = t
}

func (a *Album) fetchOriginal(key originalCacheKey) ([]byte, error) {
	// TODO(dan): Add timeout, Download gets stuck.
	filename := key.Filename
	log.Printf("album: fetching %s", filename)
	resp, err := a.dropbox.Files.Download(&dropbox.DownloadInput{
		Path: path.Join(a.folder, filename),
	})
	if err != nil {
		return []byte{}, err
	}
	return ioutil.ReadAll(resp.Body)
}

func (a *Album) fetchThumbnail(key thumbCacheKey) ([]byte, error) {
	data, err := a.cache.Get(originalCacheKey{key.Filename})
	if err != nil {
		return []byte{}, err
	}
	log.Printf("album: resizing %s", key.Filename)
	return Resize(data, key.Width, key.Height)
}

type originalCacheKey struct {
	Filename string
}

func (o originalCacheKey) String() string {
	return o.Filename
}

type thumbCacheKey struct {
	Filename string
	Width    uint
	Height   uint
}

func (t thumbCacheKey) Dependencies() []interface{} {
	return []interface{}{originalCacheKey{t.Filename}}
}

func (t thumbCacheKey) String() string {
	return fmt.Sprintf("%s@%dx%d", t.Filename, t.Width, t.Height)
}
