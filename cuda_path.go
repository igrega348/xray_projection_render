//go:build linux && cgo

package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"

	"github.com/go-gl/mathgl/mgl64"
	"github.com/igrega348/xray_projection_render/objects"
	"github.com/rs/zerolog/log"
)

// cudaRenderImages renders all projections on the GPU using the preloaded VoxelGrid in lat[0].
// Populates transform_params.Frames, writes PNGs, and writes transforms_file.
func cudaRenderImages(
	camera_angles []CameraAngle,
	res int,
	R, fov, ds_param float64,
	transforms_file, output_dir, fname_pattern string,
	transform_params *TransformParams,
	time_label float64,
	transparency bool,
) error {
	vg, ok := lat[0].(*objects.VoxelGrid)
	if !ok {
		return fmt.Errorf("--use_cuda requires a voxel_grid input, got %T", lat[0])
	}

	// Convert float64 Rho to float32 for CUDA.
	vol_f32 := make([]float32, len(vg.Rho))
	for i, v := range vg.Rho {
		vol_f32[i] = float32(v)
	}

	// Auto ds: one step per voxel (divided by 5 for sub-voxel accuracy, matching CPU convention).
	ds := ds_param
	if ds < 0 {
		// Use the finest voxel dimension so all axes are sub-voxel sampled.
		minDim := vg.NX
		if vg.NY < minDim {
			minDim = vg.NY
		}
		if vg.NZ < minDim {
			minDim = vg.NZ
		}
		ds = 2.0 / float64(minDim) / 5.0
	}

	res_f := float64(res)
	f := 1 / math.Tan(mgl64.DegToRad(fov/2))
	transform_params.FL_X = f * res_f / 2.0
	transform_params.FL_Y = f * res_f / 2.0

	// Build camera params and pre-populate transform frames.
	cams := make([]xrayCameraParams, len(camera_angles))
	for i_img, angle := range camera_angles {
		eye, camMat := computeCameraFromAngles(angle.Azimuthal, angle.Polar, R)

		// Row-major view matrix for CUDA transform_point().
		var view [16]float32
		for r := 0; r < 4; r++ {
			for c := 0; c < 4; c++ {
				view[r*4+c] = float32(camMat.At(r, c))
			}
		}
		cams[i_img] = xrayCameraParams{
			eye:  [3]float32{float32(eye[0]), float32(eye[1]), float32(eye[2])},
			view: view,
			fovY: float32(fov),
			R:    float32(R),
		}

		tm := make([][]float64, 4)
		for r := 0; r < 4; r++ {
			tm[r] = make([]float64, 4)
			for c := 0; c < 4; c++ {
				tm[r][c] = camMat.At(r, c)
			}
		}
		filename := filepath.Join(output_dir, fmt.Sprintf(fname_pattern, i_img))
		dname, fname := filepath.Split(filename)
		rel_path := filepath.Join(filepath.Base(dname), fname)
		transform_params.Frames = append(transform_params.Frames, OneFrameParams{
			FilePath:        filepath.ToSlash(rel_path),
			TransformMatrix: tm,
			Time:            time_label,
		})
	}

	// Batch-render all cameras on GPU.
	outImages := make([]float32, len(camera_angles)*res*res)
	log.Info().Msgf("CUDA render: %d cameras, res=%d, vol=%dx%dx%d, ds=%f", len(camera_angles), res, vg.NX, vg.NY, vg.NZ, ds)
	if err := renderVolumeCUDA(vol_f32, vg.NX, vg.NY, vg.NZ, cams, res, ds, flat_field, outImages); err != nil {
		return err
	}

	// Write PNGs.
	for i_img := range camera_angles {
		myImage := image.NewRGBA(image.Rect(0, 0, res, res))
		base := i_img * res * res
		for i := 0; i < res; i++ {
			for j := 0; j < res; j++ {
				val := float64(outImages[base+i*res+j])
				var alpha uint16
				if transparency {
					if val < 1.0 {
						alpha = 0xffff
					}
				} else {
					alpha = 0xffff
				}
				c := color.RGBA64{uint16(val * 0xffff), uint16(val * 0xffff), uint16(val * 0xffff), alpha}
				myImage.SetRGBA64(i, res-j-1, c)
			}
		}
		filename := filepath.Join(output_dir, fmt.Sprintf(fname_pattern, i_img))
		out, err := os.Create(filename)
		if err != nil {
			return fmt.Errorf("creating %s: %w", filename, err)
		}
		png.Encode(out, myImage)
		out.Close()
	}
	return nil
}
