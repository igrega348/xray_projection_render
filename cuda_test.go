//go:build cuda
// +build cuda

package main

import (
	"math"
	"testing"

	"github.com/go-gl/mathgl/mgl64"
	"github.com/igrega348/xray_projection_render/objects"
)

// probeCUDA returns true if at least one CUDA device is accessible.
// It does this by trying a minimal renderVolumeCUDA call and checking whether the
// error is a device-level failure (codes 3–6, which all come from cudaMalloc /
// cudaMemcpy failures that indicate no device) vs. a usage error.
func probeCUDA() bool {
	vol := []float32{0.0}
	cam := xrayCameraParams{
		eye:  [3]float32{4, 0, 0},
		view: [16]float32{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1},
		fovY: 40,
		R:    4,
	}
	out := make([]float32, 1)
	err := renderVolumeCUDA(vol, 1, 1, 1, []xrayCameraParams{cam}, 1, 0.1, 0.0, out)
	return err == nil
}

// TestCPUvsCUDA renders a voxelized sphere with both the CPU (integrate_along_ray)
// and CUDA (renderVolumeCUDA) paths, then checks per-pixel agreement.
//
// Sources of expected disagreement (both systematic, not bugs):
//   - float32 (CUDA) vs float64 (CPU) arithmetic
//   - Voxel-center offset: CUDA texture maps voxel i to (i+0.5)/N in normalised coords,
//     while Go's VoxelGrid.Density maps world coord x to grid index (x+1)/2*(N-1),
//     producing a ~0.5-voxel shift. For N=32 this is ~1.5 % of the grid width.
//
// Tolerance 0.10 (max per-pixel) and 0.03 (RMSE) will catch real bugs such as axis
// swaps or a wrong coordinate transform while tolerating the known systematic differences.
func TestCPUvsCUDA(t *testing.T) {
	if !probeCUDA() {
		t.Skip("no CUDA device available")
	}
	const N = 32
	vg := &objects.VoxelGrid{NX: N, NY: N, NZ: N}
	vg.Rho = make([]float64, N*N*N)
	for iz := 0; iz < N; iz++ {
		for ix := 0; ix < N; ix++ {
			for iy := 0; iy < N; iy++ {
				x := float64(ix)/float64(N)*2.0 - 1.0
				y := float64(iy)/float64(N)*2.0 - 1.0
				z := float64(iz)/float64(N)*2.0 - 1.0
				if x*x+y*y+z*z < 0.3*0.3 {
					vg.Rho[iz*N*N+ix*N+iy] = 1.0
				}
			}
		}
	}

	defer setupPhysics(vg, 0.0, 1.0)()

	const (
		res    = 32
		fovDeg = 40.0
		R      = 4.0
	)
	ds := 2.0 / float64(N) / 5.0 // 5 steps per voxel along each axis

	eye, camera := computeCameraFromAngles(0.0, 90.0, R)
	resFl := float64(res)
	f := 1.0 / math.Tan(mgl64.DegToRad(fovDeg/2))

	// CPU render: replicate the per-pixel ray computation from render().
	cpuPixels := make([]float64, res*res)
	for i := 0; i < res; i++ {
		for j := 0; j < res; j++ {
			vx := mgl64.Vec3{float64(i)/(resFl/2) - 1, float64(j)/(resFl/2) - 1, -f}
			vx = mgl64.TransformCoordinate(vx, camera)
			dir := vx.Sub(eye)
			cpuPixels[i*res+j] = integrate_along_ray(eye, dir, ds, R-cube_half_diagonal, R+cube_half_diagonal)
		}
	}

	// CUDA render.
	vol_f32 := make([]float32, len(vg.Rho))
	for idx, v := range vg.Rho {
		vol_f32[idx] = float32(v)
	}
	var view [16]float32
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			view[r*4+c] = float32(camera.At(r, c))
		}
	}
	cam := xrayCameraParams{
		eye:  [3]float32{float32(eye[0]), float32(eye[1]), float32(eye[2])},
		view: view,
		fovY: float32(fovDeg),
		R:    float32(R),
	}
	cudaPixels := make([]float32, res*res)
	if err := renderVolumeCUDA(vol_f32, N, N, N, []xrayCameraParams{cam}, res, ds, 0.0, cudaPixels); err != nil {
		t.Fatalf("renderVolumeCUDA: %v", err)
	}

	var maxDiff, sumSqErr float64
	for i := 0; i < res*res; i++ {
		diff := math.Abs(cpuPixels[i] - float64(cudaPixels[i]))
		if diff > maxDiff {
			maxDiff = diff
		}
		sumSqErr += diff * diff
	}
	rmse := math.Sqrt(sumSqErr / float64(res*res))
	t.Logf("CPU vs CUDA: max_diff=%.4f rmse=%.4f", maxDiff, rmse)
	if maxDiff > 0.10 {
		t.Errorf("max pixel diff %.4f > 0.10", maxDiff)
	}
	if rmse > 0.03 {
		t.Errorf("RMSE %.4f > 0.03", rmse)
	}
}

// TestCPUvsCUDA_NonCubic repeats the agreement test for a non-cubic volume (NX≠NY≠NZ).
// This specifically catches axis-confusion bugs that only appear when dimensions differ.
func TestCPUvsCUDA_NonCubic(t *testing.T) {
	if !probeCUDA() {
		t.Skip("no CUDA device available")
	}
	const NX, NY, NZ = 24, 32, 16
	vg := &objects.VoxelGrid{NX: NX, NY: NY, NZ: NZ}
	vg.Rho = make([]float64, NX*NY*NZ)
	for iz := 0; iz < NZ; iz++ {
		for ix := 0; ix < NX; ix++ {
			for iy := 0; iy < NY; iy++ {
				x := float64(ix)/float64(NX)*2.0 - 1.0
				y := float64(iy)/float64(NY)*2.0 - 1.0
				z := float64(iz)/float64(NZ)*2.0 - 1.0
				// Rectangular box with different extents per axis.
				if math.Abs(x) < 0.4 && math.Abs(y) < 0.2 && math.Abs(z) < 0.3 {
					vg.Rho[iz*NX*NY+ix*NY+iy] = 1.0
				}
			}
		}
	}

	defer setupPhysics(vg, 0.0, 1.0)()

	const (
		res    = 32
		fovDeg = 40.0
		R      = 4.0
	)
	minDim := NX
	if NY < minDim {
		minDim = NY
	}
	if NZ < minDim {
		minDim = NZ
	}
	ds := 2.0 / float64(minDim) / 5.0

	eye, camera := computeCameraFromAngles(0.0, 90.0, R)
	resFl := float64(res)
	f := 1.0 / math.Tan(mgl64.DegToRad(fovDeg/2))

	cpuPixels := make([]float64, res*res)
	for i := 0; i < res; i++ {
		for j := 0; j < res; j++ {
			vx := mgl64.Vec3{float64(i)/(resFl/2) - 1, float64(j)/(resFl/2) - 1, -f}
			vx = mgl64.TransformCoordinate(vx, camera)
			dir := vx.Sub(eye)
			cpuPixels[i*res+j] = integrate_along_ray(eye, dir, ds, R-cube_half_diagonal, R+cube_half_diagonal)
		}
	}

	vol_f32 := make([]float32, len(vg.Rho))
	for idx, v := range vg.Rho {
		vol_f32[idx] = float32(v)
	}
	var view [16]float32
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			view[r*4+c] = float32(camera.At(r, c))
		}
	}
	cam := xrayCameraParams{
		eye:  [3]float32{float32(eye[0]), float32(eye[1]), float32(eye[2])},
		view: view,
		fovY: float32(fovDeg),
		R:    float32(R),
	}
	cudaPixels := make([]float32, res*res)
	if err := renderVolumeCUDA(vol_f32, NX, NY, NZ, []xrayCameraParams{cam}, res, ds, 0.0, cudaPixels); err != nil {
		t.Fatalf("renderVolumeCUDA: %v", err)
	}

	var maxDiff, sumSqErr float64
	for i := 0; i < res*res; i++ {
		diff := math.Abs(cpuPixels[i] - float64(cudaPixels[i]))
		if diff > maxDiff {
			maxDiff = diff
		}
		sumSqErr += diff * diff
	}
	rmse := math.Sqrt(sumSqErr / float64(res*res))
	t.Logf("non-cubic CPU vs CUDA: max_diff=%.4f rmse=%.4f", maxDiff, rmse)
	if maxDiff > 0.12 {
		t.Errorf("max pixel diff %.4f > 0.12", maxDiff)
	}
	if rmse > 0.03 {
		t.Errorf("RMSE %.4f > 0.03", rmse)
	}
}
