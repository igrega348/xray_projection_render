package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-gl/mathgl/mgl64"
	"github.com/igrega348/sphere_render/objects"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/schollz/progressbar/v3"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v3"
)

const flat_field = 0.0

// func make_object() objects.Lattice {
// 	var kelvin_uc = objects.MakeKelvin(0.075, 0.8)
// 	// return kelvin_uc.Tesselate(4, 4, 4)
// 	return kelvin_uc
// }

// func load_object() objects.Lattice {
// 	fn := "lattice.yaml"
// 	data, err := os.ReadFile(fn)
// 	if err != nil {
// 		fmt.Println("Error reading file:", err)
// 	}
// 	out := map[string]interface{}{}
// 	err = yaml.Unmarshal(data, &out)
// 	if err != nil {
// 		fmt.Println("Error unmarshalling YAML to map", err)
// 	}
// 	lat := objects.Lattice{}
// 	err = lat.FromMap(out)
// 	if err != nil {
// 		fmt.Println("Error converting to lattice:", err)
// 	}
// 	if out["tessellate"] != nil {
// 		nx := out["tessellate"].([]interface{})[0].(int)
// 		ny := out["tessellate"].([]interface{})[1].(int)
// 		nz := out["tessellate"].([]interface{})[2].(int)
// 		lat = lat.Tesselate(nx, ny, nz)
// 	}
// 	fmt.Println("Lattice loaded")
// 	return lat
// }

func load_object(fn string) objects.ObjectCollection {
	log.Info().Msgf("Loading object from '%s'", fn)
	data, err := os.ReadFile(fn)
	if err != nil {
		log.Fatal().Err(err)
	}
	out := map[string]interface{}{}
	// can have either yaml or json based on file extension via switch
	switch ext := fn[len(fn)-4:]; ext {
	case "yaml":
		err = yaml.Unmarshal(data, &out)
		if err != nil {
			log.Error().Msgf("Error unmarshalling YAML: %v", err)
		}
	case "json":
		err = json.Unmarshal(data, &out)
		if err != nil {
			log.Error().Msgf("Error unmarshalling JSON: %v", err)
		}
	default:
		fmt.Println("Unknown file extension:", ext)
	}
	objcoll := objects.ObjectCollection{}
	err = objcoll.FromMap(out)
	if err != nil {
		log.Error().Msgf("Error converting to object collection: %v", err)
	}
	return objcoll
}

func make_object() objects.TessellatedObjColl {
	uc := objects.MakeKelvin(0.03, 0.5)
	lat := objects.TessellatedObjColl{UC: uc, Xmin: -1.0, Xmax: 1.0, Ymin: -1.0, Ymax: 1.0, Zmin: -1.0, Zmax: 1.0}
	return lat
}

var lat = objects.ObjectCollection{}
var integrate = integrate_along_ray

func deform(x, y, z float64) (float64, float64, float64) {
	// Try Gaussian displacement field
	A := 0.05
	sigma := 0.2
	y = y - A*math.Exp(-(x*x+y*y+z*z)/(2*sigma*sigma))
	return x, y, z
}

func density(x, y, z float64) float64 {
	// x, y, z = deform(x, y, z)
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

func integrate_hierarchical(origin, direction mgl64.Vec3, DS, smin, smax float64) float64 {
	// normalize components of the ray
	direction = direction.Normalize()
	// check clipping
	if density(origin[0]+direction[0]*smin, origin[1]+direction[1]*smin, origin[2]+direction[2]*smin) > 0 {
		log.Warn().Msg("Clipping at smin detected")
	}
	if density(origin[0]+direction[0]*smax, origin[1]+direction[1]*smax, origin[2]+direction[2]*smax) > 0 {
		log.Warn().Msg("Clipping at smax detected")
	}
	// integrate using sliding window
	right := smin + DS
	left := smin
	ds := DS / 10.0
	prev_rho := 0.0
	T := flat_field
	for right <= smax {
		x := origin[0] + direction[0]*right
		y := origin[1] + direction[1]*right
		z := origin[2] + direction[2]*right
		rho := density(x, y, z)
		if (rho == 0) != (prev_rho == 0) { // rho changed between left and right
			left += ds
			for left < right {
				x := origin[0] + direction[0]*left
				y := origin[1] + direction[1]*left
				z := origin[2] + direction[2]*left
				T += density(x, y, z) * ds
				left += ds
			}
			T += rho * ds // reuse rho from right
		} else {
			T += rho * DS
		}
		prev_rho = rho
		left = right
		right += DS
	}
	return math.Exp(-T)
}

func computePixel(img [][]float64, i, j int, origin, direction mgl64.Vec3, ds, smin, smax float64, wg *sync.WaitGroup) {
	defer wg.Done()
	// img[i][j] = integrate_along_ray(origin, direction, ds, smin, smax)
	// img[i][j] = integrate_hierarchical(origin, direction, ds, smin, smax)
	img[i][j] = integrate(origin, direction, ds, smin, smax)
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
	W           int        `json:"w"`
	H           int        `json:"h"`
	CX          float64    `json:"cx"`
	CY          float64    `json:"cy"`
	Frames      []OneParam `json:"frames"`
}

func render(
	input string,
	output_dir string,
	fname_pattern string,
	res int,
	num_images int,
	out_of_plane bool,
	ds float64,
	R float64,
	fov float64,
) {
	defer timer()()

	lat = load_object(input)
	// create output directory if it doesn't exist
	if _, err := os.Stat(output_dir); os.IsNotExist(err) {
		log.Info().Msgf("Creating output directory '%s'", output_dir)
		os.Mkdir(output_dir, 0755)
	} else {
		log.Info().Msgf("Output to directory '%s'", output_dir)
	}
	// set or compute ds
	if ds < 0 {
		ds = lat.MinFeatureSize() / 10.0
		log.Info().Msgf("Setting ds to %f", ds)
	}

	if out_of_plane {
		log.Info().Msg("Random polar angle")
	} else {
		log.Info().Msg("Fixed polar angle at 90 degrees")
	}
	log.Info().Msgf("Generating %d images at resolution %d", num_images, res)
	res_f := float64(res)

	img := make([][]float64, res)
	for i := range img {
		img[i] = make([]float64, res) // [0.0, 0.0, ... 0.0
	}

	transform_params := TransformParams{
		CameraAngle: fov * math.Pi / 180.0,
		W:           res,
		H:           res,
		CX:          res_f / 2.0,
		CY:          res_f / 2.0,
		Frames:      []OneParam{},
	}
	min_val, max_val := 1.0, 0.0

	bar := progressbar.Default(int64(num_images))
	for i_img := 0; i_img < num_images; i_img++ {
		dth := 360.0 / float64(num_images)
		var th, phi float64

		th = float64(i_img) * dth
		if out_of_plane {
			// phi random
			z := rand.Float64()*2 - 1
			phi = math.Acos(z)
		} else {
			phi = math.Pi / 2.0
		}
		bar.Add(1)
		// zero out img
		for i := 0; i < res; i++ {
			for j := 0; j < res; j++ {
				img[i][j] = 0
			}
		}

		eye := mgl64.Vec3{R * math.Cos(mgl64.DegToRad(float64(th))) * math.Sin(phi), R * math.Sin(mgl64.DegToRad(float64(th))) * math.Sin(phi), math.Cos(phi) * R}
		center := mgl64.Vec3{0, 0, 0}
		up := mgl64.Vec3{0, 0, 1}
		camera := mgl64.LookAtV(eye, center, up)
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
		transform_params.FL_X = f * res_f / 2.0
		transform_params.FL_Y = f * res_f / 2.0
		for i := 0; i < res; i++ {
			for j := 0; j < res; j++ {
				wg.Add(1)
				vx := mgl64.Vec3{float64(i)/(res_f/2) - 1, float64(j)/(res_f/2) - 1, -f}
				vx = mgl64.TransformCoordinate(vx, camera)
				go computePixel(img, i, j, eye, vx.Sub(eye), ds, R-1.41, R+1.41, &wg)
			}
		}
		wg.Wait()

		myImage := image.NewRGBA(image.Rect(0, 0, res, res))
		for i := 0; i < res; i++ {
			for j := 0; j < res; j++ {
				val := img[i][j]
				c := color.RGBA64{uint16(val * 0xffff), uint16(val * 0xffff), uint16(val * 0xffff), 0xffff}
				// image has origin at top left, so we need to flip the y coordinate
				myImage.SetRGBA64(i, res-j, c)
				if val < min_val {
					min_val = val
				}
				if val > max_val {
					max_val = val
				}
			}
		}
		if i_img == 0 || i_img == num_images-1 {
			log.Info().Msgf("Min value: %f, Max value: %f", min_val, max_val)
		}
		// Save to out.png
		filename := filepath.Join(output_dir, fmt.Sprintf(fname_pattern, i_img))
		out, err := os.Create(filename)
		if err != nil {
			log.Panic().Err(err)
		}
		log.Debug().Msgf("Saving image to '%s'", filename)
		png.Encode(out, myImage)
		out.Close()

		transform_params.Frames = append(transform_params.Frames, OneParam{FilePath: filename, TransformMatrix: rows})
	}

	// write transform parameters to JSON
	jsonData, err := json.MarshalIndent(transform_params, "", "  ")
	if err != nil {
		fmt.Println("Error marshalling to JSON:", err)
	}
	log.Info().Msg("Writing transform parameters to 'transforms.json'")
	err = os.WriteFile("transforms.json", jsonData, 0644)
	if err != nil {
		fmt.Println("Error writing JSON to file:", err)
	}

	// write object to JSON or YAML
	// data, err := json.MarshalIndent(lat.ToMap(), "", "  ")
	data, err := yaml.Marshal(lat.ToMap())
	if err != nil {
		fmt.Println("Error marshalling object:", err)
	}
	log.Info().Msg("Writing object to 'object.yaml'")
	err = os.WriteFile("object.yaml", data, 0644)
	if err != nil {
		fmt.Println("Error writing file:", err)
	}
}

func main() {
	app := &cli.App{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "output_dir",
				Usage: "Output directory to save the images",
				Value: "images",
			},
			&cli.StringFlag{
				Name:     "input",
				Usage:    "Input yaml file describing the object",
				Required: true,
			},
			&cli.IntFlag{
				Name:  "num_projections",
				Usage: "Number of projections to generate",
				Value: 16,
			},
			&cli.IntFlag{
				Name:  "resolution",
				Usage: "Resolution of the square output images",
				Value: 512,
			},
			&cli.BoolFlag{
				Name:  "out_of_plane",
				Usage: "Generate out of plane projections",
			},
			&cli.StringFlag{
				Name:  "fname_pattern",
				Usage: "Sprintf pattern for output file name",
				Value: "image_%03d.png",
			},
			&cli.Float64Flag{
				Name:  "ds",
				Usage: "Integration step size. If negative, try to infer from smallest feature size in the input file",
				Value: -1.0,
			},
			&cli.Float64Flag{
				Name:  "R",
				Usage: "Distance between camera and centre of scene",
				Value: 5.0,
			},
			&cli.Float64Flag{
				Name:  "fov",
				Usage: "Field of view in degrees",
				Value: 45.0,
			},
			&cli.StringFlag{
				Name: "integration",
				Usage: "Integration method to use. Options are 'simple' or 'hierarchical'. " +
					"Simple method integrates along the ray in fixed steps. " +
					"Hierarchical method uses a sliding window to integrate along the ray.",
				Value: "hierarchical",
			},
			// verbose flag
			&cli.BoolFlag{
				Name:  "v",
				Usage: "Enable verbose logging",
			},
		},
		Action: func(cCtx *cli.Context) error {
			log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
			if cCtx.Bool("v") {
				zerolog.SetGlobalLevel(zerolog.InfoLevel)
			} else {
				zerolog.SetGlobalLevel(zerolog.WarnLevel)
			}
			if cCtx.String("integration") == "simple" {
				integrate = integrate_along_ray
				log.Info().Msg("Using simple integration method")
			} else if cCtx.String("integration") == "hierarchical" {
				integrate = integrate_hierarchical
				log.Info().Msg("Using hierarchical integration method")
			} else {
				log.Fatal().Msgf("Unknown integration method: %s", cCtx.String("integration"))
			}
			render(
				cCtx.String("input"),
				cCtx.String("output_dir"),
				cCtx.String("fname_pattern"),
				cCtx.Int("resolution"),
				cCtx.Int("num_projections"),
				cCtx.Bool("out_of_plane"),
				cCtx.Float64("ds"),
				cCtx.Float64("R"),
				cCtx.Float64("fov"),
			)
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal().Err(err)
	}
}
