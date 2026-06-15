//go:build cuda
// +build cuda

package main

/*
#cgo CFLAGS: -I.
#cgo LDFLAGS: -L${SRCDIR} -lcuda_render -L/usr/local/cuda-13.0/targets/x86_64-linux/lib -lcudart -Wl,-rpath,${SRCDIR} -Wl,-rpath,/usr/local/cuda-13.0/targets/x86_64-linux/lib

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

// cylinderParams mirrors CylinderParams for Go.
type cylinderParams struct {
	p0, p1 [3]float32
	radius float32
	rho    float32
}

func (p *cylinderParams) toC() C.CylinderParams {
	var c C.CylinderParams
	for i := 0; i < 3; i++ {
		c.p0[i] = C.float(p.p0[i])
		c.p1[i] = C.float(p.p1[i])
	}
	c.radius = C.float(p.radius)
	c.rho = C.float(p.rho)
	return c
}

// assembleVoxelGridCUDA voxelizes a cylinder scene on the GPU.
// Returns a float32 volume of length res³ with layout [k*res*res + i*res + j].
func assembleVoxelGridCUDA(cylinders []cylinderParams, res int, densityMultiplier float64) ([]float32, error) {
	if len(cylinders) == 0 {
		return nil, fmt.Errorf("assembleVoxelGridCUDA: no cylinders provided")
	}
	cCyls := make([]C.CylinderParams, len(cylinders))
	for i := range cylinders {
		cCyls[i] = cylinders[i].toC()
	}
	total := res * res * res
	out := make([]float32, total)
	ret := C.AssembleVoxelGridCUDA(
		(*C.CylinderParams)(&cCyls[0]),
		C.int(len(cylinders)),
		C.int(res),
		C.float(densityMultiplier),
		(*C.float)(&out[0]),
	)
	if int(ret) != 0 {
		return nil, fmt.Errorf("AssembleVoxelGridCUDA returned error code %d", int(ret))
	}
	return out, nil
}

// assembleVoxelGridSpatialCUDA builds a CSR spatial-hash on CPU, then voxelizes on GPU.
// gridDim controls the spatial hash resolution (e.g. 16 → 16³ = 4096 cells).
func assembleVoxelGridSpatialCUDA(cylinders []cylinderParams, res, gridDim int, densityMultiplier float64) ([]float32, error) {
	if len(cylinders) == 0 {
		return nil, fmt.Errorf("assembleVoxelGridSpatialCUDA: no cylinders")
	}
	numCells := gridDim * gridDim * gridDim
	cellSize := 2.0 / float64(gridDim)

	// Build cylinder index lists per cell using AABB overlap.
	cellLists := make([][]int, numCells)
	for ci, cyl := range cylinders {
		// AABB for this cylinder.
		ax, bx := float64(cyl.p0[0]), float64(cyl.p1[0])
		ay, by := float64(cyl.p0[1]), float64(cyl.p1[1])
		az, bz := float64(cyl.p0[2]), float64(cyl.p1[2])
		r := float64(cyl.radius)
		xmin := min64(ax, bx) - r
		xmax := max64(ax, bx) + r
		ymin := min64(ay, by) - r
		ymax := max64(ay, by) + r
		zmin := min64(az, bz) - r
		zmax := max64(az, bz) + r

		// Grid cells overlapped by AABB.
		cxMin := clampGrid(int((xmin+1.0)/cellSize), gridDim)
		cxMax := clampGrid(int((xmax+1.0)/cellSize), gridDim)
		cyMin := clampGrid(int((ymin+1.0)/cellSize), gridDim)
		cyMax := clampGrid(int((ymax+1.0)/cellSize), gridDim)
		czMin := clampGrid(int((zmin+1.0)/cellSize), gridDim)
		czMax := clampGrid(int((zmax+1.0)/cellSize), gridDim)

		for cz := czMin; cz <= czMax; cz++ {
			for cy := cyMin; cy <= cyMax; cy++ {
				for cx := cxMin; cx <= cxMax; cx++ {
					cell := (cz*gridDim+cy)*gridDim + cx
					cellLists[cell] = append(cellLists[cell], ci)
				}
			}
		}
	}

	// Build CSR: cell_offsets and cyl_indices.
	cellOffsets := make([]int32, numCells+1)
	var totalAssign int
	for c := 0; c < numCells; c++ {
		cellOffsets[c] = int32(totalAssign)
		totalAssign += len(cellLists[c])
	}
	cellOffsets[numCells] = int32(totalAssign)

	cylIndices := make([]int32, totalAssign)
	var pos int
	for c := 0; c < numCells; c++ {
		for _, ci := range cellLists[c] {
			cylIndices[pos] = int32(ci)
			pos++
		}
	}

	// Prepare C arrays.
	cCyls := make([]C.CylinderParams, len(cylinders))
	for i := range cylinders {
		cCyls[i] = cylinders[i].toC()
	}
	total := res * res * res
	out := make([]float32, total)

	var cOffPtr *C.int
	var cIdxPtr *C.int
	if len(cellOffsets) > 0 {
		cOffPtr = (*C.int)(&cellOffsets[0])
	}
	if len(cylIndices) > 0 {
		cIdxPtr = (*C.int)(&cylIndices[0])
	}

	ret := C.AssembleVoxelGridSpatialCUDA(
		(*C.CylinderParams)(&cCyls[0]),
		C.int(len(cylinders)),
		C.int(res),
		C.float(densityMultiplier),
		C.int(gridDim),
		cOffPtr,
		cIdxPtr,
		C.int(totalAssign),
		(*C.float)(&out[0]),
	)
	if int(ret) != 0 {
		return nil, fmt.Errorf("AssembleVoxelGridSpatialCUDA returned error code %d", int(ret))
	}
	return out, nil
}

func min64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
func max64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
func clampGrid(v, dim int) int {
	if v < 0 {
		return 0
	}
	if v >= dim {
		return dim - 1
	}
	return v
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

