# DBPS

A Go library for creating Dropbox backed photo sites.


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
  http.Handle("/images/", http.StripPrefix("/images/", p.PhotoHandler))

  log.Fatal(http.ListenAndServe(":8080", nil))
}
```

Contributing
------------
This is not really intended for mass use, but if you do have questions,
comments, bug reports, and pull requests please submit them
[on the project issue tracker](https://github.com/dpup/dbps/issues/new).

License
-------
Copyright 2015 [Daniel Pupius](http://pupius.co.uk). Licensed under the
[Apache License, Version 2.0](http://www.apache.org/licenses/LICENSE-2.0).
