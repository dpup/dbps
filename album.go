// Copyright 2015 Daniel Pupius

package dbps

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"path"
	"sort"
	"sync"
	"time"

	"github.com/dpup/dbps/fetcher"
	"github.com/dpup/dbps/internal/dropbox"
	"github.com/dpup/dbps/internal/goexif/exif"
)

// Album queries dropbox and keeps a list of photos in date order.
type Album struct {
	folder  string
	dropbox *dropbox.Dropbox
	fetcher *fetcher.Fetcher

	lastHash  string
	photoList PhotoList
	photoMap  map[string]Photo
	loading   bool
	mu        sync.RWMutex
}

func NewAlbum(folder string, dropbox *dropbox.Dropbox, fetcher *fetcher.Fetcher) *Album {
	return &Album{folder: folder, dropbox: dropbox, fetcher: fetcher}
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

	entry, err := a.dropbox.Metadata(a.folder, true, false, a.lastHash, "", 5000)

	if dbError, ok := err.(*dropbox.Error); ok && dbError.StatusCode == 304 {
		log.Println("album: no metadata changes detected")
		return nil
	} else if err != nil {
		return fmt.Errorf("album: failed to get metadata: %s", err)
	}

	if !entry.IsDir {
		return errors.New("album: provided path was not a directory")
	}

	log.Println("album: loading image metadata")

	var wg sync.WaitGroup

	photos := make(PhotoList, len(entry.Contents))
	for i, e := range entry.Contents {
		name := path.Base(e.Path)
		clientModified := time.Time(e.ClientMtime)
		dropboxModified := time.Time(e.Modified)

		// e.Hash is empty so use own approximation.
		hash := fmt.Sprintf("%d:%d:%d", e.Bytes, clientModified.Unix(), dropboxModified.Unix())

		// If no entry exists, or the entry is stale, then load the photo to get its
		// exif data. Loads are done in parallel.
		if old, ok := a.photoMap[name]; !ok || old.Hash != hash {
			photos[i] = Photo{
				Filename:        name,
				MimeType:        e.MimeType,
				Size:            e.Bytes,
				Hash:            hash,
				DropboxModified: dropboxModified,
				ExifCreated:     clientModified, // Default to the last modified time.
			}

			wg.Add(1)
			a.fetcher.Remove(name)
			go a.loadExifInfo(&photos[i], &wg)

		} else {
			photos[i] = old
		}
	}

	log.Printf("album: waiting for new images to load")
	wg.Wait()
	sort.Sort(photos)

	// TODO(dan): Currently we are not clearing the cache of deleted images, for
	// the existing usecase that is a rare scenario. Can easily be added by
	// asking for deleted items and checking entry.IsDeleted

	a.mu.Lock()
	a.lastHash = entry.Hash
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

// Photo returns the metadata for a photo or an error if it doesn't exist.
func (a *Album) Photo(name string) (Photo, []byte, error) {
	if photo, ok := a.photoMap[name]; ok {
		data, err := a.fetcher.Get(name)
		return photo, data, err
	} else {
		return Photo{}, nil, fmt.Errorf("album: no photo with name: %s", name)
	}
}

// Photos returns a copy of the PhotoList.
func (a *Album) Photos() PhotoList {
	a.mu.RLock()
	defer a.mu.RUnlock()
	c := make(PhotoList, len(a.photoList))
	copy(c, a.photoList)
	return c
}

func (a *Album) loadExifInfo(p *Photo, wg *sync.WaitGroup) {
	defer func() { wg.Done() }()

	data, err := a.fetcher.Get(p.Filename)
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
	log.Printf("album: loaded %s", p)
}