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

func Resize(data []byte, w, h uint) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nilBytes, err
	}

	img = Crop(w, h, Cover(w, h, img))

	var buf bytes.Buffer
	err = jpeg.Encode(&buf, img, &jpeg.Options{95})
	if err != nil {
		return nilBytes, err
	}

	return buf.Bytes(), nil
}

// Cover resizes an image such that it will cover a space of sie (w x h) with no
// letter boxing. Resultant image is not cropped, so will overflow the target
// size unless the aspect ratio exactly matches.
func Cover(w, h uint, img image.Image) image.Image {
	bounds := img.Bounds()
	if bounds.Dx()*int(h) < bounds.Dy()*int(w) {
		h = 0
	} else {
		w = 0
	}
	return resize.Resize(w, h, img, resize.Bicubic)
}

// Crop will return an image of size (w, h) centered on the provided image.
func Crop(w, h uint, img image.Image) image.Image {
	b := img.Bounds()
	x := int(float64(b.Dx())/2 - float64(w)/2)
	y := int(float64(b.Dy())/2 - float64(h)/2)

	return img.(interface {
		SubImage(r image.Rectangle) image.Image
	}).SubImage(image.Rect(x, y, x+int(w), y+int(h)))
}
