package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"sync"
	"time"

	"github.com/go-gl/mathgl/mgl64"
)

const res = 512
const f = 2.0

func density(x, y, z float64) float64 {
	x0, y0, z0 := 0.0, 0.0, 3.0
	x = x - x0
	y = y - y0
	z = z - z0
	// r := math.Sqrt((x * x) + (y * y) + (z * z))
	// if r < 0.25 {
	// 	return 0.0
	// } else if r < 0.75 {
	// 	return 0.01
	// } else {
	// 	return 0
	// }
	// cube
	if x < 0.5 && x > -0.5 && y < 0.5 && y > -0.5 && z < 0.5 && z > -0.5 {
		return 0.01
	} else {
		return 0
	}
}

func integrate_along_ray(vx, vy, vz, ds, smin, smax float64) float64 {
	// normalize components of the ray
	norm := math.Sqrt(vx*vx + vy*vy + vz*vz)
	vx /= norm
	vy /= norm
	vz /= norm
	// integrate
	T := 0.0
	for s := smin; s < smax; s += ds {
		x := vx * s
		y := vy * s
		z := vz * s
		T += density(x, y, z) * ds
	}
	return math.Exp(-T)
}

func computePixel(img *[res][res]float64, i, j int, x, y, z float64, ds, smin, smax float64, wg *sync.WaitGroup) {
	defer wg.Done()
	img[i][j] = integrate_along_ray(x, y, z, ds, smin, smax)
}

func timer() func() {
	start := time.Now()
	return func() {
		fmt.Println(time.Since(start))
	}
}

func main() {
	defer timer()()
	var img [res][res]float64

	projection := mgl64.Perspective(mgl64.DegToRad(45.0), 1, 0.1, 10)
	camera := mgl64.LookAt(3, 3, 3, 0, 0, 0, 0, 1, 0)
	fmt.Println(projection)
	fmt.Println(camera)

	// concurrent version:
	var wg sync.WaitGroup
	for i := 0; i < res; i++ {
		for j := 0; j < res; j++ {
			wg.Add(1)
			vx := mgl64.Vec3{float64(i)/(res/2) - 1, float64(j)/(res/2) - 1, f}
			vx = mgl64.TransformCoordinate(vx, projection.Mul4(camera))
			go computePixel(&img, i, j, vx.X(), vx.Y(), vx.Z(), 0.001, 2, 5, &wg)
		}
	}
	wg.Wait()

	_max, _min := 0.0, 1.0
	for i := 0; i < res; i++ {
		for j := 0; j < res; j++ {
			// non-concurrent version:
			// img[i][j] = integrate_along_ray(float64(i)/(res/2)-1, float64(j)/(res/2)-1, 2.0, 0.001, 0, 5)
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
			// c = color.RGBA64{uint16(val * 0xffff), uint16(val * 0xffff), uint16(val * 0xffff), 0xffff}
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
