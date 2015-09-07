// Copyright 2015 Daniel Pupius

package dbps

import (
	"bytes"
	"image"
	"image/jpeg"

	_ "image/gif"
	_ "image/png"

	"github.com/dpup/dbps/internal/resize"
)

var nilBytes = []byte{}

func Resize(data []byte) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nilBytes, err
	}

	r := resize.Thumbnail(200, 200, img, resize.Bicubic)

	var b bytes.Buffer
	err = jpeg.Encode(&b, r, &jpeg.Options{95})
	if err != nil {
		return nilBytes, err
	}

	return b.Bytes(), nil
}
