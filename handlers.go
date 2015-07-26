// Copyright 2015 Daniel Pupius

package dbps

import (
	"bytes"
	"encoding/json"
	"net/http"
)

// Writes the photo data as JSON.
type jsonHandler struct {
	album *Album
}

func (j *jsonHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Cache-Control", "max-age=180, public, must-revalidate, proxy-revalidate")
	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	js, _ := json.Marshal(struct {
		Photos photoList
	}{
		Photos: j.album.Photos(),
	})
	w.Write(js)
}

// Writes an image to the response.
type photoHandler struct {
	album *Album
}

func (p *photoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	photo, data, err := p.album.Photo(r.URL.Path)
	if err != nil {
		// TODO(dan): Nicer error pages.
		http.Error(w, err.Error(), 500)
	} else {
		w.Header().Add("Cache-Control", "max-age=864000, public, must-revalidate, proxy-revalidate")
		http.ServeContent(w, r, photo.Filename, photo.DropboxModified, bytes.NewReader(data))
	}
}
