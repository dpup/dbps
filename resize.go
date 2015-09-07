// Copyright 2015 Daniel Pupius

package dbps

import (
	"bytes"
	"image"
	"image/jpeg"
	"math"

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

	b := img.Bounds()
	s := math.Min(float64(b.Dx()), float64(b.Dy()))
	x := int(float64(b.Dx())/2 - s/2)
	y := int(float64(b.Dy())/2 - s/2)

	subImg := img.(interface {
		SubImage(r image.Rectangle) image.Image
	})

	squareImg := subImg.SubImage(image.Rect(x, y, x+int(s), y+int(s)))
	resizedImg := resize.Thumbnail(200, 200, squareImg, resize.Bicubic)

	var buf bytes.Buffer
	err = jpeg.Encode(&buf, resizedImg, &jpeg.Options{95})
	if err != nil {
		return nilBytes, err
	}

	return buf.Bytes(), nil
}
