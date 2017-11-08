// Copyright 2015 Daniel Pupius

package dbps

import (
	"fmt"
	"time"
)

// Metadata for the photo.
type Photo struct {
	Filename        string
	Size            int
	Hash            string    `json:"-"`
	DropboxModified time.Time `json:"-"`
	ExifCreated     time.Time
}

func (p *Photo) String() string {
	return fmt.Sprintf("%s (%s)", p.Filename, p.ExifCreated)
}

// Array of photos, sortable by the Exif created time.
type photoList []Photo

func (a photoList) Len() int      { return len(a) }
func (a photoList) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a photoList) Less(i, j int) bool {
	return a[j].ExifCreated.Before(a[i].ExifCreated)
}
