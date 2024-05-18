package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"sync"
	"testing"

	"github.com/go-gl/mathgl/mgl64"
	"github.com/pkg/profile"
)

func TestMain(m *testing.M) {
	defer profile.Start().Stop()
	const res = 128
	const num_images = 2
	const R = 4.0
	const fov = 45.0
	var img = make([][]float64, res)
	for i := range img {
		img[i] = make([]float64, res)
	}

	// create a progress bar
	for i_img := 0; i_img < num_images; i_img++ {
		dth := 360.0 / num_images
		var th, phi float64

		th = float64(i_img) * dth
		phi = math.Pi / 2.0
		// zero out img
		for i := 0; i < res; i++ {
			for j := 0; j < res; j++ {
				img[i][j] = 0
			}
		}

		origin := mgl64.Vec3{R * math.Cos(mgl64.DegToRad(float64(th))) * math.Sin(phi), R * math.Sin(mgl64.DegToRad(float64(th))) * math.Sin(phi), math.Cos(phi) * R}
		center := mgl64.Vec3{0, 0, 0}
		up := mgl64.Vec3{0, 0, 1}
		camera := mgl64.LookAtV(origin, center, up)
		// try to use the matrix to transform coordinates from camera space to world space
		camera = camera.Inv()

		rows := make([][]float64, 4)
		for i := 0; i < 4; i++ {
			rows[i] = make([]float64, 4)
			for j := 0; j < 4; j++ {
				rows[i][j] = camera.At(i, j)
			}
		}

		var wg sync.WaitGroup
		f := 1 / math.Tan(mgl64.DegToRad(fov/2))
		for i := 0; i < res; i++ {
			for j := 0; j < res; j++ {
				wg.Add(1)
				vx := mgl64.Vec3{float64(i)/(res/2) - 1, float64(j)/(res/2) - 1, -f}
				vx = mgl64.TransformCoordinate(vx, camera)
				go computePixel(img, i, j, origin, vx.Sub(origin), 0.001, R-1.0, R+1.0, &wg)
			}
		}
		wg.Wait()

		myImage := image.NewRGBA(image.Rect(0, 0, res, res))
		for i := 0; i < res; i++ {
			for j := 0; j < res; j++ {
				val := img[i][j]
				c := color.RGBA64{uint16(val * 0xffff), uint16(val * 0xffff), uint16(val * 0xffff), 0xffff}
				myImage.SetRGBA64(i, j, c)
			}
		}
		// Save to out.png
		filename := fmt.Sprintf("pics/out%d.png", i_img)
		out, err := os.Create(filename)
		if err != nil {
			panic(err)
		}
		png.Encode(out, myImage)
		out.Close()

	}
}
