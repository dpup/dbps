// Copyright 2015 Daniel Pupius

// Package dbps provides a utility for creating Dropbox backed photo sites. The
// library queries a Dropbox folder for files, caches images in memory, and
// polls for new images. HTTP handlers are provided that serve the images and
// returns a list of photos as JSON.
package dbps

import (
	"log"
	"net/http"
	"time"

	"github.com/dpup/dbps/internal/dropbox"
)

// Config defines how to access the site.
type Config struct {
	DropBoxAccessToken string
	PhotoFolder        string
	PollFreq           time.Duration
}

// PhotoSite provides functionality for binding to your own server mux.
type PhotoSite struct {
	DataHandler      http.Handler
	PhotoHandler     http.Handler
	ThumbnailHandler http.Handler
	Album            *Album
}

// NewPhotoSite fetches data about a photo album from DropBox and monitors for changes.
func NewPhotoSite(config Config) *PhotoSite {
	d := dropbox.New(dropbox.NewConfig(config.DropBoxAccessToken))
	album := NewAlbum(config.PhotoFolder, d)

	pf := time.Second * 30
	if config.PollFreq > 0 {
		pf = config.PollFreq
	}

	// TODO(dan): Come up with a better way of loading and polling for changes.
	// This loads all the images, in order to get EXIF data, which has the side
	// effect of pre-warming teh cache.
	go func() {
		err := album.Load()
		if err != nil {
			log.Fatal(err)
		}
		album.Monitor(pf)
	}()

	return &PhotoSite{
		&jsonHandler{album},
		&photoHandler{album},
		&thumbnailHandler{album},
		album,
	}
}
