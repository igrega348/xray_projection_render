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
)

const res = 1024
const fov = 45.0
const R = 3.0
const rad = 0.1
const rad_2 = rad * rad
const num_images = 5
const flat_field = 0.0

// func density(x, y, z float64) float64 {
// 	points := [][]float64{
// 		{0.5, 0.5, 0.5},
// 		{0.5, 0.5, -0.5},
// 		{0.5, -0.5, 0.5},
// 		{0.5, -0.5, -0.5},
// 		{-0.5, 0.5, 0.5},
// 		{-0.5, 0.5, -0.5},
// 		{-0.5, -0.5, 0.5},
// 		{-0.5, -0.5, -0.5},
// 	}
// 	lines := [][]int{
// 		{0, 1},
// 		{0, 2},
// 		{0, 4},
// 		{1, 3},
// 		{1, 5},
// 		{2, 3},
// 		{2, 6},
// 		{3, 7},
// 		{4, 5},
// 		{4, 6},
// 		{5, 7},
// 		{6, 7},
// 	}
// 	u := []float64{x, y, z}
// 	rad_2 := rad * rad
// 	for _, p := range lines {
// 		p0 := points[p[0]]
// 		p1 := points[p[1]]
// 		// get the vector from the point to the line
// 		v := []float64{p1[0] - p0[0], p1[1] - p0[1], p1[2] - p0[2]}
// 		w := []float64{u[0] - p0[0], u[1] - p0[1], u[2] - p0[2]}
// 		// get the projection of w onto v
// 		// c := w.Dot(v) / v.Dot(v)
// 		c := (w[0]*v[0] + w[1]*v[1] + w[2]*v[2]) / (v[0]*v[0] + v[1]*v[1] + v[2]*v[2])
// 		if c < 0.0 || c > 1.0 { // point is definitely not on the line
// 			continue
// 		}
// 		// get the distance from the point to the line
// 		// d := w.Sub(v.Mul(c)).Len()
// 		d := c * ((w[0]-v[0])*(w[0]-v[0]) + (w[1]-v[1])*(w[1]-v[1]) + (w[2]-v[2])*(w[2]-v[2]))
// 		if d < rad_2 {
// 			return 1.0
// 		}
// 	}
// 	return 0.0
// }

func density(x, y, z float64) float64 {
	x0, y0, z0 := 0.0, 0.0, 0.0
	x = x - x0
	y = y - y0
	z = z - z0
	// sphere
	r := math.Sqrt((x * x) + (y * y) + (z * z))
	if r < 0.25 {
		return 0.0
	} else if r < 0.75 {
		return 1.0
	} else {
		return 0
	}
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

func computePixel(img *[res][res]float64, i, j int, origin, direction mgl64.Vec3, ds, smin, smax float64, wg *sync.WaitGroup) {
	defer wg.Done()
	// img[i][j] = integrate_adaptive(origin, direction, smin, smax)
	// img[i][j] = integrate_hierarchical(origin, direction, ds, smin, smax)
	// img[i][j] = integrate_along_ray(origin, direction, ds, smin, smax)
	points := [][]float64{
		{0.5, 0.5, 0.5},
		{0.5, 0.5, -0.5},
		{0.5, -0.5, 0.5},
		{0.5, -0.5, -0.5},
		{-0.5, 0.5, 0.5},
		{-0.5, 0.5, -0.5},
		{-0.5, -0.5, 0.5},
		{-0.5, -0.5, -0.5},
	}
	lines := [][]int{
		{0, 1},
		{0, 2},
		{0, 4},
		{1, 3},
		{1, 5},
		{2, 3},
		{2, 6},
		{3, 7},
		{4, 5},
		{4, 6},
		{5, 7},
		{6, 7},
	}
	intersection_length := 0.0
	// for now ignore the issue of intersecting cylinders
	// intersection_enter := make([]float64, len(points))
	// intersection_exit := make([]float64, len(points))
	for _, p := range lines {
		p0 := points[p[0]]
		p1 := points[p[1]]
		v := mgl64.Vec3{p1[0] - p0[0], p1[1] - p0[1], p1[2] - p0[2]}
		w := mgl64.Vec3{p0[0] - origin[0], p0[1] - origin[1], p0[2] - origin[2]}
		n := direction.Cross(v)
		// var min_s, max_s float64
		if n.Len() < 0.0001 {
			// if lines are parallel
			// starting at p0, go to p1
			// min_s = w.Dot(direction)
			// max_s = w.Add(v).Dot(direction)
			intersection_length += v.Len()
		} else {
			n_unit := n.Normalize()
			// get the projection of w onto n
			c := w.Dot(n_unit)
			c_2 := c * c
			if c > rad_2 {
				// not intersecting the line
				// min_s = math.Inf(-1)
				// max_s = math.Inf(-1)
				continue
			} else {
				// need to check if the ray intersects the cylinder within the line segment
				ni := direction.Cross(n_unit)
				eta_j := -w.Dot(ni) / v.Dot(ni)
				if eta_j < 0 || eta_j > 1 {
					continue
				}
				// what are the two values of s where the ray intersects the cylinder?
				// s = (c +/- sqrt(rad^2 - c^2)) / direction
				s := math.Sqrt(rad_2 - c_2)
				intersection_length += 2 * s
				// min_s = direction.Dot(w) - s
				// max_s = direction.Dot(w) + s
			}
		}
		// if min_s > max_s {
		// 	min_s, max_s = max_s, min_s
		// }
		// intersection_enter[i] = min_s
		// intersection_exit[i] = max_s
	}
	img[i][j] = math.Exp(-2 * intersection_length)
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
