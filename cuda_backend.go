//go:build linux && cgo

package main

/*
#cgo CFLAGS: -I.
#cgo LDFLAGS: -ldl

#include <dlfcn.h>
#include <stdio.h>
#include <stdlib.h>
#include "cuda_backend.h"

typedef int (*fn_AssembleVoxelGridCUDA_t)(const CylinderParams*, int, int, float, float*);
typedef int (*fn_AssembleVoxelGridSpatialCUDA_t)(const CylinderParams*, int, int, float, int, const int*, const int*, int, float*);
typedef int (*fn_RenderVolumeProjectionsCUDA_t)(const float*, int, int, int, const XRayCameraParams*, int, int, float, float, float*);

static void*                              s_cuda_lib        = NULL;
static fn_AssembleVoxelGridCUDA_t         s_assemble        = NULL;
static fn_AssembleVoxelGridSpatialCUDA_t  s_assembleSpatial = NULL;
static fn_RenderVolumeProjectionsCUDA_t   s_render          = NULL;
// s_error_buf owns the error string so it is valid across CGO thread boundaries.
// dlerror() returns a pointer into thread-local storage that is only valid until
// the next dlerror()/dl*() call on the same OS thread; strdup() into a stable buffer.
static char s_error_buf[512] = "library not loaded";

static int cuda_dl_load(const char* path) {
    const char* e;
    dlerror();
    s_cuda_lib = dlopen(path, RTLD_LAZY | RTLD_LOCAL);
    if (!s_cuda_lib) {
        e = dlerror(); if (e) snprintf(s_error_buf, sizeof(s_error_buf), "%s", e);
        return -1;
    }
    s_assemble = (fn_AssembleVoxelGridCUDA_t)dlsym(s_cuda_lib, "AssembleVoxelGridCUDA");
    if (!s_assemble) { e = dlerror(); if (e) snprintf(s_error_buf, sizeof(s_error_buf), "%s", e); return -2; }
    s_assembleSpatial = (fn_AssembleVoxelGridSpatialCUDA_t)dlsym(s_cuda_lib, "AssembleVoxelGridSpatialCUDA");
    if (!s_assembleSpatial) { e = dlerror(); if (e) snprintf(s_error_buf, sizeof(s_error_buf), "%s", e); return -2; }
    s_render = (fn_RenderVolumeProjectionsCUDA_t)dlsym(s_cuda_lib, "RenderVolumeProjectionsCUDA");
    if (!s_render) { e = dlerror(); if (e) snprintf(s_error_buf, sizeof(s_error_buf), "%s", e); return -2; }
    s_error_buf[0] = '\0';
    return 0;
}

static const char* cuda_dl_last_error(void) {
    return s_error_buf;
}

static int cuda_dl_AssembleVoxelGridCUDA(
        const CylinderParams* cyls, int n, int res, float dm, float* out) {
    if (!s_assemble) return -99;
    return s_assemble(cyls, n, res, dm, out);
}

static int cuda_dl_AssembleVoxelGridSpatialCUDA(
        const CylinderParams* cyls, int n, int res, float dm,
        int gdim, const int* offsets, const int* indices, int nidx, float* out) {
    if (!s_assembleSpatial) return -99;
    return s_assembleSpatial(cyls, n, res, dm, gdim, offsets, indices, nidx, out);
}

static int cuda_dl_RenderVolumeProjectionsCUDA(
        const float* vol, int nx, int ny, int nz,
        const XRayCameraParams* cams, int ncams,
        int ires, float ds, float ff, float* out) {
    if (!s_render) return -99;
    return s_render(vol, nx, ny, nz, cams, ncams, ires, ds, ff, out);
}
*/
import "C"

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"unsafe"
)

var (
	cudaLoadOnce sync.Once
	cudaLoadErr  error
)

// initCUDA lazily loads libcuda_render.so on first call.
// Subsequent calls return the cached result.
func initCUDA() error {
	cudaLoadOnce.Do(func() {
		lib := findCUDALib()
		clib := C.CString(lib)
		defer C.free(unsafe.Pointer(clib))
		if ret := C.cuda_dl_load(clib); ret != 0 {
			cudaLoadErr = fmt.Errorf("CUDA library not available (%s): %s", lib, C.GoString(C.cuda_dl_last_error()))
		}
	})
	return cudaLoadErr
}

// findCUDALib returns the path to try for libcuda_render.so:
// 1. XRAY_CUDA_LIB env var (explicit override)
// 2. Same directory as the running executable
// 3. Bare name (resolved via LD_LIBRARY_PATH / ldconfig)
func findCUDALib() string {
	if p := os.Getenv("XRAY_CUDA_LIB"); p != "" {
		return p
	}
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), "libcuda_render.so")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "libcuda_render.so"
}

// xrayCameraParams mirrors the C struct XRayCameraParams for Go.
type xrayCameraParams struct {
	eye  [3]float32
	view [16]float32 // row-major order
	fovY float32
	R    float32
}

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

func assembleVoxelGridCUDA(cylinders []cylinderParams, res int, densityMultiplier float64) ([]float32, error) {
	if err := initCUDA(); err != nil {
		return nil, err
	}
	if len(cylinders) == 0 {
		return nil, fmt.Errorf("assembleVoxelGridCUDA: no cylinders provided")
	}
	cCyls := make([]C.CylinderParams, len(cylinders))
	for i := range cylinders {
		cCyls[i] = cylinders[i].toC()
	}
	total := res * res * res
	out := make([]float32, total)
	ret := C.cuda_dl_AssembleVoxelGridCUDA(
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
func assembleVoxelGridSpatialCUDA(cylinders []cylinderParams, res, gridDim int, densityMultiplier float64) ([]float32, error) {
	if err := initCUDA(); err != nil {
		return nil, err
	}
	if len(cylinders) == 0 {
		return nil, fmt.Errorf("assembleVoxelGridSpatialCUDA: no cylinders")
	}
	numCells := gridDim * gridDim * gridDim
	cellSize := 2.0 / float64(gridDim)

	cellLists := make([][]int, numCells)
	for ci, cyl := range cylinders {
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

	ret := C.cuda_dl_AssembleVoxelGridSpatialCUDA(
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

func renderVolumeCUDA(
	volume []float32,
	nx, ny, nz int,
	cameras []xrayCameraParams,
	imageRes int,
	ds float64,
	flatField float64,
	outImages []float32,
) error {
	if err := initCUDA(); err != nil {
		return err
	}
	if len(volume) != nx*ny*nz {
		return fmt.Errorf("renderVolumeCUDA: volume length %d does not match dimensions %d x %d x %d", len(volume), nx, ny, nz)
	}
	if len(outImages) != len(cameras)*imageRes*imageRes {
		return fmt.Errorf("renderVolumeCUDA: outImages length %d does not match expected %d", len(outImages), len(cameras)*imageRes*imageRes)
	}
	if len(cameras) == 0 {
		return fmt.Errorf("renderVolumeCUDA: no cameras provided")
	}

	cCams := make([]C.XRayCameraParams, len(cameras))
	for i := range cameras {
		cCams[i] = cameras[i].toC()
	}

	ret := C.cuda_dl_RenderVolumeProjectionsCUDA(
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
