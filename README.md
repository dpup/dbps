# DBPS

A Go library for creating Dropbox backed photo sites.

The library queries a Dropbox folder for files, caches images in memory, and
polls for new images. HTTP Handlers are provided to serve the image files and to
return a list of photos as JSON (ordered by EXIF created at time).

This is used by a couple of my sites. It is not supported and offered as-is.

Example
-------

```go
package main

import (
  "log"
  "net/http"

  "github.com/dpup/dbps"
)

func main() {

  p := dbps.NewPhotoSite(dbps.Config{
    DropBoxClientID:     "[redacted]",
    DropBoxClientSecret: "[redacted]",
    DropBoxAccessToken:  "[redacted]",
    PhotoFolder:         "Photos/my-portfolio",
  })

  http.Handle("/photos.json", p.DataHandler)
  http.Handle("/photos/", http.StripPrefix("/photos/", p.PhotoHandler))
  http.Handle("/thumbnails/", http.StripPrefix("/thumbnails/", p.ThumbnailHandler))

  log.Fatal(http.ListenAndServe(":8080", nil))
}
```

Generate tokens using Dropbox's [App Console](https://www.dropbox.com/developers/apps).

Contributing
------------
This is not really intended for mass use, but if you do have questions,
comments, bug reports, and pull requests please submit them
[on the project issue tracker](https://github.com/dpup/dbps/issues/new).

License
-------
Copyright 2015 [Daniel Pupius](http://pupius.co.uk). Licensed under the
[Apache License, Version 2.0](http://www.apache.org/licenses/LICENSE-2.0).
