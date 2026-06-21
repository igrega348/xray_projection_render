//go:build !linux || !cgo

package main

import "errors"

func cudaRenderImages(
	camera_angles []CameraAngle,
	res int,
	R, fov, ds_param float64,
	transforms_file, output_dir, fname_pattern string,
	transform_params *TransformParams,
	time_label float64,
	transparency bool,
) error {
	return errors.New("--use_cuda is only supported on Linux (amd64) with CGO enabled")
}
