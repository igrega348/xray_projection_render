package main

import (
	"encoding/json"
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
const fov = 45.0
const R = 3.0
const num_images = 4
const flat_field = 0.0

func density(x, y, z float64) float64 {
	points := make([]mgl64.Vec2, 4)
	points[0] = mgl64.Vec2{-0.5, -0.5}
	points[1] = mgl64.Vec2{0.5, -0.5}
	points[2] = mgl64.Vec2{0.5, 0.5}
	points[3] = mgl64.Vec2{-0.5, 0.5}
	if z < -0.5 || z > 0.5 {
		return 0.0
	}
	for i := 0; i < 4; i++ {
		r := mgl64.Vec2{x - points[i][0], y - points[i][1]}
		if r.Len() < 0.1 {
			return 1.0
		}
	}
	return 0.0
}

// func density(x, y, z float64) float64 {
// 	x0, y0, z0 := 0.0, 0.0, 0.0
// 	x = x - x0
// 	y = y - y0
// 	z = z - z0
// 	// sphere
// 	r := math.Sqrt((x * x) + (y * y) + (z * z))
// 	if r < 0.25 {
// 		return 0.0
// 	} else if r < 0.75 {
// 		return 1.0
// 	} else {
// 		return 0
// 	}
// 	// cube
// 	// if x < 0.5 && x > -0.5 && y < 0.5 && y > -0.5 && z < 0.5 && z > -0.5 {
// 	// 	return 0.01
// 	// } else {
// 	// 	return 0
// 	// }
// 	// cube with a spherical hole
// 	// r := math.Sqrt((x*x + y*y + z*z))
// 	// if r < 0.25 {
// 	// 	return 0.0
// 	// }
// 	// if x < 0.5 && x > -0.5 && y < 0.5 && y > -0.5 && z < 0.5 && z > -0.5 {
// 	// 	return 1.0
// 	// }
// 	// return 0
// 	// sphere with a cubic hole
// 	// r := math.Sqrt((x*x + y*y + z*z))
// 	// if math.Abs(x) < 0.25 && math.Abs(y) < 0.25 && math.Abs(z) < 0.25 {
// 	// 	return 0.0
// 	// } else if r < 0.5 {
// 	// 	return 1.0
// 	// } else {
// 	// 	return 0.0
// 	// }
// }

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

type OneParam struct {
	FilePath        string      `json:"file_path"`
	TransformMatrix [][]float64 `json:"transform_matrix"`
}
type TransformParams struct {
	CameraAngle float64    `json:"camera_angle_x"`
	FL_X        float64    `json:"fl_x"`
	FL_Y        float64    `json:"fl_y"`
	W           float64    `json:"w"`
	H           float64    `json:"h"`
	CX          float64    `json:"cx"`
	CY          float64    `json:"cy"`
	Frames      []OneParam `json:"frames"`
}

func main() {
	defer timer()()
	var img [res][res]float64

	transform_params := TransformParams{
		CameraAngle: fov * math.Pi / 180.0,
		W:           res,
		H:           res,
		CX:          res / 2.0,
		CY:          res / 2.0,
		Frames:      []OneParam{},
	}
	// create a progress bar
	bar := progressbar.Default(int64(num_images))
	for i_img := 0; i_img < num_images; i_img++ {
		dth := 360.0 / num_images
		var th, phi float64
		// if i_img < 4 {
		// 	th = 90.0*float64(i_img%4) + 45.0
		// 	phi = math.Pi / 2.0
		// } else if i_img < 6 {
		// 	th = 0
		// 	if i_img == 4 {
		// 		phi = 0.0001
		// 	} else {
		// 		phi = math.Pi
		// 	}
		// } else {
		// 	th = rand.Float64() * 360.0
		// 	phi = rand.Float64() * math.Pi
		// }

		th = float64(i_img) * dth
		phi = math.Pi / 2.0
		bar.Add(1)
		// zero out img
		for i := 0; i < res; i++ {
			for j := 0; j < res; j++ {
				img[i][j] = 0
			}
		}

		// angle from vertical
		// phi := 90.0 * math.Pi / 180.0
		// z := rand.Float64()*2 - 1
		// phi := math.Acos(z)
		// spiral trajectory
		// z := 0.99 - 1.98*float64(i_img)/float64(num_images-1)
		// phi := math.Acos(z)

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
		transform_params.FL_X = f * res / 2.0
		transform_params.FL_Y = f * res / 2.0
		for i := 0; i < res; i++ {
			for j := 0; j < res; j++ {
				wg.Add(1)
				vx := mgl64.Vec3{float64(i)/(res/2) - 1, float64(j)/(res/2) - 1, -f}
				vx = mgl64.TransformCoordinate(vx, camera)
				go computePixel(&img, i, j, origin, vx.Sub(origin), 0.001, R-1.0, R+1.0, &wg)
			}
		}
		wg.Wait()

		myImage := image.NewRGBA(image.Rect(0, 0, res, res))
		for i := 0; i < res; i++ {
			for j := 0; j < res; j++ {
				val := img[i][j]
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
		filename := fmt.Sprintf("pics/out%d.png", i_img)
		out, err := os.Create(filename)
		if err != nil {
			panic(err)
		}
		png.Encode(out, myImage)
		out.Close()

		transform_params.Frames = append(transform_params.Frames, OneParam{FilePath: filename, TransformMatrix: rows})
	}

	// Optionally, write JSON data to a file
	file, err := os.Create("transforms.json")
	if err != nil {
		fmt.Println("Error creating file:", err)
		return
	}
	defer file.Close()

	jsonData, err := json.MarshalIndent(transform_params, "", "  ")
	_, err = file.Write(jsonData)
}
