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
	"github.com/schollz/progressbar/v3"
	"gonum.org/v1/gonum/integrate/quad"
)

const res = 512
const fov = 10.0
const R = 10.0
const num_images = 2
const flat_field = 0.1

func density(x, y, z float64) float64 {
	x0, y0, z0 := 0.0, 0.0, 0.0
	x = x - x0
	y = y - y0
	z = z - z0
	// sphere
	// r := math.Sqrt((x * x) + (y * y) + (z * z))
	// if r < 0.25 {
	// 	return 0.0
	// } else if r < 0.75 {
	// 	return 1.0
	// } else {
	// 	return 0
	// }
	// cube
	// if x < 0.5 && x > -0.5 && y < 0.5 && y > -0.5 && z < 0.5 && z > -0.5 {
	// 	return 0.01
	// } else {
	// 	return 0
	// }
	// cube with a spherical hole
	r := math.Sqrt((x*x + y*y + z*z))
	if r < 0.25 {
		return 0.0
	}
	if x < 0.5 && x > -0.5 && y < 0.5 && y > -0.5 && z < 0.5 && z > -0.5 {
		return 1.0
	}
	return 0
}

func integrate_along_ray(origin, direction mgl64.Vec3, ds, smin, smax float64) float64 {
	// normalize components of the ray
	direction = direction.Normalize()
	// integrate
	T := flat_field
	for s := smin; s < smax; s += ds {
		x := origin[0] + direction[0]*s
		y := origin[1] + direction[1]*s
		z := origin[2] + direction[2]*s
		T += density(x, y, z) * ds
	}
	return math.Exp(-T)
}

func integrate_hierarchical(origin, direction mgl64.Vec3, ds, smin, smax float64) float64 {
	// normalize components of the ray
	direction = direction.Normalize()
	// integrate
	T := 0.0
	for s := smin; s <= smax; s += 9 * ds {
		x := origin[0] + direction[0]*s
		y := origin[1] + direction[1]*s
		z := origin[2] + direction[2]*s
		rho := density(x, y, z)
		if rho > 0 {
			for _s := s - 4*ds; _s <= s+4*ds; _s += ds {
				x := origin[0] + direction[0]*_s
				y := origin[1] + direction[1]*_s
				z := origin[2] + direction[2]*_s
				T += density(x, y, z) * ds
			}
		}
	}
	return math.Exp(-T)
}

func integrate_adaptive(origin, direction mgl64.Vec3, smin, smax float64) float64 {
	// normalize components of the ray
	direction = direction.Normalize()
	f := func(s float64) float64 {
		x := origin[0] + direction[0]*s
		y := origin[1] + direction[1]*s
		z := origin[2] + direction[2]*s
		return density(x, y, z)
	}
	return math.Exp(-adaptive_quad(f, smin, smax, 100))
}

func adaptive_quad(f func(float64) float64, xmin, xmax float64, n int) float64 {
	// integrate f from xmin to xmax
	// split into 2 intervals and evaluate the integral of each interval
	xmid := (xmin + xmax) / 2
	// n2 := int(math.Ceil(float64(n) / 2))
	n2 := (n + 1) / 2 // should be the same as above
	int1 := quad.Fixed(f, xmin, xmid, n2, nil, 0)
	int2 := quad.Fixed(f, xmid, xmax, n2, nil, 0)
	// new resolution
	_int1 := quad.Fixed(f, xmin, xmid, n, nil, 0)
	_int2 := quad.Fixed(f, xmid, xmax, n, nil, 0)
	thresh := 1e-5
	if math.Abs(_int1-int1) > thresh {
		_int1 = adaptive_quad(f, xmin, xmid, n)
	}
	if math.Abs(_int2-int2) > thresh {
		_int2 = adaptive_quad(f, xmid, xmax, n)
	}
	return _int1 + _int2
}

func computePixel(img *[res][res]float64, i, j int, origin, direction mgl64.Vec3, ds, smin, smax float64, wg *sync.WaitGroup) {
	defer wg.Done()
	// img[i][j] = integrate_adaptive(origin, direction, smin, smax)
	// img[i][j] = integrate_hierarchical(origin, direction, ds, smin, smax)
	img[i][j] = integrate_along_ray(origin, direction, ds, smin, smax)
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

	bar := progressbar.Default(int64(num_images))
	for i_img := 0; i_img < num_images; i_img++ {
		dth := 180.0 / (num_images - 1)
		th := float64(i_img)*dth + 10.0
		bar.Add(1)
		// zero out img
		for i := 0; i < res; i++ {
			for j := 0; j < res; j++ {
				img[i][j] = 0
			}
		}

		origin := mgl64.Vec3{R * math.Cos(mgl64.DegToRad(float64(th))), R * math.Sin(mgl64.DegToRad(float64(th))), 0}
		center := mgl64.Vec3{0, 0, 0}
		up := mgl64.Vec3{0, 0, 1}
		camera := mgl64.LookAtV(origin, center, up)
		// try to use the matrix to transform coordinates from camera space to world space
		camera = camera.Inv()

		var wg sync.WaitGroup
		f := 1 / math.Tan(mgl64.DegToRad(fov/2))
		for i := 0; i < res; i++ {
			for j := 0; j < res; j++ {
				wg.Add(1)
				vx := mgl64.Vec3{float64(i)/(res/2) - 1, float64(j)/(res/2) - 1, -f}
				vx = mgl64.TransformCoordinate(vx, camera)
				go computePixel(&img, i, j, origin, vx.Sub(origin), 0.001, R-1.0, R+1.0, &wg)
			}
		}
		wg.Wait()

		// if th == 0 {
		// 	_max, _min := 0.0, 1.0
		// 	for i := 0; i < res; i++ {
		// 		for j := 0; j < res; j++ {
		// 			if img[i][j] > _max {
		// 				_max = img[i][j]
		// 			}
		// 			if img[i][j] < _min {
		// 				_min = img[i][j]
		// 			}
		// 		}
		// 	}
		// 	fmt.Println(_max, _min)
		// }

		myImage := image.NewRGBA(image.Rect(0, 0, res, res))
		for i := 0; i < res; i++ {
			for j := 0; j < res; j++ {
				val := img[i][j]
				// val := (img[i][j] - _min) / (_max - _min)
				c := color.RGBA64{uint16(val * 0xffff), uint16(val * 0xffff), uint16(val * 0xffff), 0xffff}
				// var c color.RGBA64
				// if val == 1 {
				// 	c = color.RGBA64{0, 0, 0, 0x0000}
				// } else {
				// 	c = color.RGBA64{uint16(val * 0xffff), uint16(val * 0xffff), uint16(val * 0xffff), 0xffff}
				// }
				myImage.SetRGBA64(i, j, c)
			}
		}
		// Save to out.png
		out, err := os.Create(fmt.Sprintf("pics/out%.1f.png", th))
		if err != nil {
			panic(err)
		}
		png.Encode(out, myImage)
		out.Close()
	}
}
