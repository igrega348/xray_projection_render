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

	"github.com/igrega348/sphere_render/lattices"
)

const res = 200
const fov = 45.0
const R = 3.0
const rad = 0.1
const num_images = 1
const flat_field = 0.0

func make_lattice() lattices.Lattice {
	var kelvin_uc = lattices.MakeKelvin(rad)
	var struts = kelvin_uc.Struts
	nx := 4
	ny := 4
	nz := 4
	scaler := 1.0 / float64(max(nx, ny, nz))
	dx := mgl64.Vec3{1, 0, 0}
	dy := mgl64.Vec3{0, 1, 0}
	dz := mgl64.Vec3{0, 0, 1}
	var tess = make([]lattices.Strut, nx*ny*nz*len(struts))
	for i := 0; i < nx; i++ {
		for j := 0; j < ny; j++ {
			for k := 0; k < nz; k++ {
				for i_s := 0; i_s < len(struts); i_s++ {
					tess[(i*ny*nz+j*nz+k)*len(struts)+i_s] = lattices.Strut{
						P0: struts[i_s].P0.Add(dx.Mul(float64(i)).Add(dy.Mul(float64(j)).Add(dz.Mul(float64(k))))).Mul(scaler).Sub(mgl64.Vec3{0.5, 0.5, 0.5}),
						P1: struts[i_s].P1.Add(dx.Mul(float64(i)).Add(dy.Mul(float64(j)).Add(dz.Mul(float64(k))))).Mul(scaler).Sub(mgl64.Vec3{0.5, 0.5, 0.5}),
						R:  struts[i_s].R * scaler}
				}
			}
		}
	}
	return lattices.Lattice{Struts: tess}
}

var lat = make_lattice()

func deform(x, y, z float64) (float64, float64, float64) {
	// Try Gaussian displacement field
	A := 0.05
	sigma := 0.2
	y = y - A*math.Exp(-(x*x+y*y+z*z)/(2*sigma*sigma))
	return x, y, z
}

func density(x, y, z float64) float64 {
	x, y, z = deform(x, y, z)
	return lat.Density(x, y, z)
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
			T += rho * ds // central sample
			for _s := s - 4*ds; _s < s; _s += ds {
				x := origin[0] + direction[0]*_s
				y := origin[1] + direction[1]*_s
				z := origin[2] + direction[2]*_s
				T += density(x, y, z) * ds
			}
			for _s := s + ds; _s <= s+4*ds; _s += ds {
				x := origin[0] + direction[0]*_s
				y := origin[1] + direction[1]*_s
				z := origin[2] + direction[2]*_s
				T += density(x, y, z) * ds
			}
		}
	}
	return math.Exp(-T)
}

func computePixel(img *[res][res]float64, i, j int, origin, direction mgl64.Vec3, ds, smin, smax float64, wg *sync.WaitGroup) {
	defer wg.Done()
	// img[i][j] = integrate_along_ray(origin, direction, ds, smin, smax)
	img[i][j] = integrate_hierarchical(origin, direction, ds, smin, smax)
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

		th = float64(i_img) * dth
		phi = math.Pi / 2.0
		bar.Add(1)
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
