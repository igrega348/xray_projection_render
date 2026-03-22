//go:build cuda
// +build cuda

package main

/*
#cgo CFLAGS: -I.

// Linker flags for the CUDA backend library.
// The actual library name/path is expected to be provided by the build
// system (e.g. via -L and -l flags when building with -tags=cuda), so we
// only keep the include and function declarations here.

#include "cuda_backend.h"
*/
import "C"

import (
	"fmt"
)

// xrayCameraParams mirrors the C struct XRayCameraParams for Go.
type xrayCameraParams struct {
	eye  [3]float32
	view [16]float32 // row-major order
	fovY float32
	R    float32
}

// toC converts the Go-side camera representation into the C struct expected
// by the CUDA backend.
func (p *xrayCameraParams) toC() C.XRayCameraParams {
	var cCam C.XRayCameraParams
	for i := 0; i < 3; i++ {
		cCam.eye[i] = C.float(p.eye[i])
	}
	for i := 0; i < 16; i++ {
		cCam.view[i] = C.float(p.view[i])
	}
	cCam.fov_y = C.float(p.fovY)
	cCam.R = C.float(p.R)
	return cCam
}

// renderVolumeCUDA is a thin Go wrapper around the CUDA volume renderer.
//
// It is intentionally low-level: callers are expected to provide a
// precomputed density volume (normalized to [-1,1]^3 in world coordinates),
// camera parameters, and an output buffer to receive the rendered images.
func renderVolumeCUDA(
	volume []float32,
	nx, ny, nz int,
	cameras []xrayCameraParams,
	imageRes int,
	ds float64,
	flatField float64,
	outImages []float32,
) error {
	if len(volume) != nx*ny*nz {
		return fmt.Errorf("renderVolumeCUDA: volume length %d does not match dimensions %d x %d x %d", len(volume), nx, ny, nz)
	}
	if len(outImages) != len(cameras)*imageRes*imageRes {
		return fmt.Errorf("renderVolumeCUDA: outImages length %d does not match expected %d", len(outImages), len(cameras)*imageRes*imageRes)
	}
	if len(cameras) == 0 {
		return fmt.Errorf("renderVolumeCUDA: no cameras provided")
	}

	// Allocate C-side camera array and populate it.
	cCams := make([]C.XRayCameraParams, len(cameras))
	for i := range cameras {
		cCams[i] = cameras[i].toC()
	}

	// Call the CUDA backend.
	ret := C.RenderVolumeProjectionsCUDA(
		(*C.float)(&volume[0]),
		C.int(nx),
		C.int(ny),
		C.int(nz),
		(*C.XRayCameraParams)(&cCams[0]),
		C.int(len(cameras)),
		C.int(imageRes),
		C.float(ds),
		C.float(flatField),
		(*C.float)(&outImages[0]),
	)
	if int(ret) != 0 {
		return fmt.Errorf("RenderVolumeProjectionsCUDA returned error code %d", int(ret))
	}
	return nil
}

