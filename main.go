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
	"github.com/igrega348/xray_projection_render/deformations"
	"github.com/igrega348/xray_projection_render/objects"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v3"
)

var lat = []objects.Object{}
var df = []deformations.Deformation{}
var density_multiplier = 1.0
var integrate = integrate_along_ray
var flat_field = 0.0
var warned_clipping_max = false
var warned_clipping_min = false

func load_deformation(fn string) error {
	if len(fn) == 0 {
		log.Info().Msg("No deformation file provided")
		return nil
	}
	log.Info().Msgf("Loading deformation from '%s'", fn)
	data, err := os.ReadFile(fn)
	if err != nil {
		log.Fatal().Err(err)
	}
	factory := &deformations.DeformationFactory{}

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
	deformation, err := factory.Create(out)
	if err != nil {
		fmt.Println("Error creating deformation:", err)
		return err
	}
	log.Info().Msgf("Deformation: %v", deformation)
	df = append(df, deformation)
	return err
}

func load_object(fn string) error {
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
	// based on the type of object, convert to the appropriate object
	var obj objects.Object
	switch out["type"] {
	case "tessellated_obj_coll":
		obj = &objects.TessellatedObjColl{}
	case "object_collection":
		obj = &objects.ObjectCollection{}
	case "sphere":
		obj = &objects.Sphere{}
	case "cube":
		obj = &objects.Cube{}
	case "cylinder":
		obj = &objects.Cylinder{}
	default:
		log.Fatal().Msgf("Unknown object type: %v", out["type"])
	}
	err = obj.FromMap(out)
	lat = append(lat, obj)
	if err != nil {
		log.Error().Msgf("Error converting to object collection: %v", err)
	}
	return err
}

func make_object() objects.TessellatedObjColl {
	uc := objects.MakeKelvin(0.03, 0.5)
	lat := objects.TessellatedObjColl{UC: uc, Xmin: -1.02, Xmax: 1.02, Ymin: -1.02, Ymax: 1.02, Zmin: -1.02, Zmax: 1.02}
	return lat
}

func deform(x, y, z float64) (float64, float64, float64) {
	if len(df) == 0 {
		return x, y, z
	} else if len(df) == 1 {
		x, y, z = df[0].Apply(x, y, z)
		return x, y, z
	} else {
		log.Fatal().Msg("Multiple deformations not supported")
		return x, y, z
	}
}

func density(x, y, z float64) float64 {
	x, y, z = deform(x, y, z)
	return lat[0].Density(x, y, z) * density_multiplier
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
	if density(origin[0]+direction[0]*smin, origin[1]+direction[1]*smin, origin[2]+direction[2]*smin) > 0 && !warned_clipping_min {
		log.Warn().Msg("Clipping at smin detected")
		warned_clipping_min = true
	}
	if density(origin[0]+direction[0]*smax, origin[1]+direction[1]*smax, origin[2]+direction[2]*smax) > 0 && !warned_clipping_max {
		log.Warn().Msg("Clipping at smax detected")
		warned_clipping_max = true
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
	Time            float64     `json:"time"`
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
	jobs_modulo int,
	job_num int,
	transforms_file string,
	deformation_file string,
	time_label float64,
) {
	defer timer()()
	wrt := os.Stdout

	load_object(input) // modify global variable lat
	if len(lat) != 1 {
		log.Fatal().Msgf("Expected 1 object, got %d", len(lat))
	}
	err := load_deformation(deformation_file) // modify global variable df
	if err != nil {
		log.Fatal().Msgf("Error loading deformation: %v", err)
	}
	// create output directory if it doesn't exist
	if _, err := os.Stat(output_dir); os.IsNotExist(err) {
		log.Info().Msgf("Creating output directory '%s'", output_dir)
		os.Mkdir(output_dir, 0755)
	} else {
		log.Info().Msgf("Output to directory '%s'", output_dir)
	}
	// set or compute ds
	if ds < 0 {
		ds = lat[0].MinFeatureSize() / 2.0
		log.Info().Msgf("Setting ds to %f", ds)
	}

	if out_of_plane {
		log.Info().Msg("Random polar angle")
	} else {
		log.Info().Msg("Fixed polar angle at 90 degrees")
	}
	log.Info().Msgf("Generating %d images at resolution %d", num_images, res)
	log.Info().Msgf("Will render every %dth projection starting from %d", jobs_modulo, job_num)
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

	// Progress indicator
	wrt.Write([]byte("Rendering images...\n"))
	s := fmt.Sprintf("%7s%54s%6s%6s\n", "Image", "Progress", "Pix/s", "ETA")
	wrt.Write([]byte(s))
	pix_step := res * res / 50
	t0 := time.Now()

	for i_img := 0; i_img < num_images; i_img++ {
		if i_img%jobs_modulo != job_num {
			continue
		}
		s = fmt.Sprintf("%3d/%3d [", i_img, num_images)
		wrt.Write([]byte(s))

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

		t1 := time.Now()
		var wg sync.WaitGroup
		f := 1 / math.Tan(mgl64.DegToRad(fov/2))
		transform_params.FL_X = f * res_f / 2.0
		transform_params.FL_Y = f * res_f / 2.0
		for i := 0; i < res; i++ {
			for j := 0; j < res; j++ {
				wg.Add(1)
				vx := mgl64.Vec3{float64(i)/(res_f/2) - 1, float64(j)/(res_f/2) - 1, -f}
				vx = mgl64.TransformCoordinate(vx, camera)
				go computePixel(img, i, j, eye, vx.Sub(eye), ds, R-1.74, R+1.74, &wg)
				if (i*res+j)%(pix_step) == 0 {
					wrt.Write([]byte("-"))
				}
			}
		}
		wg.Wait()

		// progress indicator
		eta := time.Since(t0) * time.Duration(num_images-i_img-1) / time.Duration(i_img+1)
		pix_per_sec := float64(res*res) / time.Since(t1).Seconds()
		s = fmt.Sprintf("] %5.0f %02d:%02d\n", pix_per_sec, int(eta.Minutes()), int(eta.Seconds())%60)
		wrt.Write([]byte(s))

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

		transform_params.Frames = append(transform_params.Frames, OneParam{FilePath: filepath.ToSlash(filename), TransformMatrix: rows, Time: time_label})
	}

	// write transform parameters to JSON
	jsonData, err := json.MarshalIndent(transform_params, "", "  ")
	if err != nil {
		fmt.Println("Error marshalling to JSON:", err)
	}
	log.Info().Msgf("Writing transform parameters to '%s'", transforms_file)
	err = os.WriteFile(transforms_file, jsonData, 0644)
	if err != nil {
		fmt.Println("Error writing JSON to file:", err)
	}

	// write object to JSON or YAML
	data, err := json.MarshalIndent(lat[0].ToMap(), "", "  ")
	// data, err := yaml.Marshal(lat[0].ToMap())
	if err != nil {
		fmt.Println("Error marshalling object:", err)
	}
	log.Info().Msg("Writing object to 'object.json'")
	err = os.WriteFile("object.json", data, 0644)
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
				Value: 1,
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
				Name:  "integration",
				Usage: "Integration method to use. Options are 'simple' or 'hierarchical'. ",
				Value: "hierarchical",
			},
			&cli.Float64Flag{
				Name:  "flat_field",
				Usage: "Flat field value to add to all pixels",
				Value: 0.0,
			},
			&cli.IntFlag{
				Name: "jobs_modulo",
				Usage: "Number of jobs which are being run independently" +
					" (e.g. jobs_modulo=4 will render every 4th projection)",
				Value: 1,
			},
			&cli.IntFlag{
				Name: "job",
				Usage: "Job number to run" +
					" (e.g. job=1 with jobs_modulo=4 will render projections 1, 5, 9, ...)",
				Value: 0,
			},
			&cli.StringFlag{
				Name:  "transforms_file",
				Usage: "Output file to save the transform parameters",
				Value: "transforms.json",
			},
			&cli.Float64Flag{
				Name:  "density_multiplier",
				Usage: "Multiply all densities by this number",
				Value: 1.0,
			},
			&cli.StringFlag{
				Name:  "deformation_file",
				Usage: "File containing deformation parameters",
				Value: "",
			},
			&cli.Float64Flag{
				Name:  "time_label",
				Usage: "Label to pass to image metadata",
				Value: 0.0,
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
			flat_field = cCtx.Float64("flat_field")
			density_multiplier = cCtx.Float64("density_multiplier")
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
				cCtx.Int("jobs_modulo"),
				cCtx.Int("job"),
				cCtx.String("transforms_file"),
				cCtx.String("deformation_file"),
				cCtx.Float64("time_label"),
			)
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal().Err(err)
	}
}
