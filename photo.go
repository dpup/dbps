// Copyright 2015 Daniel Pupius

package dbps

import (
	"fmt"
	"math"
	"time"
)

// Metadata for the photo.
type Photo struct {
	Filename        string
	MimeType        string
	Size            int
	Hash            string    `json:"-"`
	DropboxModified time.Time `json:"-"`
	ExifCreated     time.Time
}

// Returns the age of the photo in days since a time (using client time).
func (p *Photo) Days(since time.Time) int {
	return int(math.Floor(p.ExifCreated.Sub(since).Hours() / 24))
}

func (p *Photo) String() string {
	return fmt.Sprintf("%s (%s)", p.Filename, p.ExifCreated)
}

// Array of photos, sortable by the Exif created time.
type PhotoList []Photo

func (a PhotoList) Len() int      { return len(a) }
func (a PhotoList) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a PhotoList) Less(i, j int) bool {
	return a[j].ExifCreated.Before(a[i].ExifCreated)
}
