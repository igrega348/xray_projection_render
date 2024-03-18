package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

func density(x, y, z float64) float64 {
	r := math.Sqrt((x * x) + (y * y) + (z * z))
	if r < 0.75 {
		return 0.01
	} else {
		return 0
	}
}

func main() {
	const res = 256
	var img [res][res]float64
	_max, _min := 0.0, 1.0
	for i := 0; i < res; i++ {
		for j := 0; j < res; j++ {
			x := float64(i)/(res/2) - 1
			y := float64(j)/(res/2) - 1
			T := 0.0
			for k := 0; k < res; k++ {
				z := float64(k)/(res/2) - 1
				dz := 2.0 / res
				T += density(x, y, z) * dz
			}
			img[i][j] = math.Exp(-T)
			if img[i][j] > _max {
				_max = img[i][j]
			}
			if img[i][j] < _min {
				_min = img[i][j]
			}
		}
	}

	myImage := image.NewRGBA(image.Rect(0, 0, res, res))
	for i := 0; i < res; i++ {
		for j := 0; j < res; j++ {
			val := (img[i][j] - _min) / (_max - _min)
			var c color.RGBA64
			if val == 1 {
				c = color.RGBA64{0, 0, 0, 0x0000}
			} else {
				c = color.RGBA64{uint16(val * 0xffff), uint16(val * 0xffff), uint16(val * 0xffff), 0xffff}
			}
			myImage.SetRGBA64(i, j, c)
		}
	}
	// Save to out.png
	out, err := os.Create("out.png")
	if err != nil {
		panic(err)
	}
	png.Encode(out, myImage)
	out.Close()
}
