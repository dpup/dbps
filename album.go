// Copyright 2015 Daniel Pupius

package dbps

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"path"
	"sort"
	"sync"
	"time"

	"github.com/dpup/dbps/cache"
	"github.com/dpup/dbps/internal/dropbox"
	"github.com/dpup/dbps/internal/goexif/exif"

	"github.com/dpup/dbps/internal/bimg"
)

// Album queries dropbox and keeps a list of photos in date order.
type Album struct {
	folder    string
	dropbox   *dropbox.Dropbox
	original  *cache.Cache
	thumbnail *cache.Cache

	lastHash  string
	photoList photoList
	photoMap  map[string]Photo
	loading   bool
	mu        sync.RWMutex
}

func NewAlbum(folder string, dropbox *dropbox.Dropbox) *Album {
	a := &Album{folder: folder, dropbox: dropbox}
	a.original = cache.New(folder+" : original", a.fetchOriginal)
	a.thumbnail = cache.New(folder+" : thumb", a.fetchThumbnail)
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

	photos := make(photoList, len(entry.Contents))
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
			a.original.Remove(name)
			a.thumbnail.Remove(name)
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

// Photo returns the metadata for a photo and the image data, or an error if it doesn't exist.
func (a *Album) Photo(name string) (Photo, []byte, error) {
	if photo, ok := a.photoMap[name]; ok {
		data, err := a.original.Get(name)
		return photo, data, err
	} else {
		return Photo{}, nil, fmt.Errorf("album: no photo with name: %s", name)
	}
}

// Thumbnail returns the metadata for a photo and a thumbnail, or an error if it doesn't exist.
func (a *Album) Thumbnail(name string) (Photo, []byte, error) {
	if photo, ok := a.photoMap[name]; ok {
		// TODO: allow variable width/height thumbnails.
		data, err := a.thumbnail.Get(name)
		return photo, data, err
	} else {
		return Photo{}, nil, fmt.Errorf("album: no photo with name: %s", name)
	}
}

// Photos returns a copy of the PhotoList.
func (a *Album) Photos() []Photo {
	a.mu.RLock()
	defer a.mu.RUnlock()
	c := make(photoList, len(a.photoList))
	copy(c, a.photoList)
	return c
}

func (a *Album) fetchOriginal(key string) ([]byte, error) {
	// TODO(dan): Add timeout, Download gets stuck.
	log.Printf("album: fetching %s", key)
	reader, _, err := a.dropbox.Download(path.Join(a.folder, key), "", 0)
	if err != nil {
		return []byte{}, err
	}
	return ioutil.ReadAll(reader)
}

func (a *Album) fetchThumbnail(key string) ([]byte, error) {
	data, err := a.original.Get(key)
	if err == nil {
		log.Printf("album: resizing %s", key)
		resized, err := bimg.NewImage(data).Process(bimg.Options{
			Width:   200,
			Height:  200,
			Embed:   true,
			Crop:    true,
			Quality: 95,
		})
		if err == nil {
			return resized, nil
		}
	}
	return []byte{}, err
}

func (a *Album) loadExifInfo(p *Photo, wg *sync.WaitGroup) {
	defer func() { wg.Done() }()

	data, err := a.original.Get(p.Filename)
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
