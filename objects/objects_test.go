package objects

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-gl/mathgl/mgl64"
)

// ── Object geometry ──────────────────────────────────────────────────────────

func TestSphereGeometry(t *testing.T) {
	s := &Sphere{Center: mgl64.Vec3{0, 0, 0}, Radius: 0.5, Rho: 2.0}

	cases := []struct {
		x, y, z float64
		want    float64
		label   string
	}{
		{0, 0, 0, 2.0, "center"},
		{0, 0, 0.4, 2.0, "inside near boundary"},
		{0, 0, 0.5, 0.0, "exactly on surface (strict <)"},
		{0, 0, 0.6, 0.0, "outside"},
	}
	for _, c := range cases {
		got := s.Density(c.x, c.y, c.z)
		if got != c.want {
			t.Errorf("Sphere.Density(%v,%v,%v) [%s] = %v, want %v", c.x, c.y, c.z, c.label, got, c.want)
		}
	}

	// off-origin center
	s2 := &Sphere{Center: mgl64.Vec3{1, 2, 3}, Radius: 0.3, Rho: 5.0}
	if got := s2.Density(1, 2, 3); got != 5.0 {
		t.Errorf("off-origin Sphere center: got %v, want 5.0", got)
	}
	if got := s2.Density(1, 2, 3.31); got != 0.0 {
		t.Errorf("off-origin Sphere outside: got %v, want 0.0", got)
	}
}

func TestBoxGeometry(t *testing.T) {
	// Box centered at origin, sides 2×1×0.5, rho 1.0
	b := &Box{Center: mgl64.Vec3{0, 0, 0}, Sides: mgl64.Vec3{2, 1, 0.5}, Rho: 1.0}

	cases := []struct {
		x, y, z float64
		want    float64
		label   string
	}{
		{0, 0, 0, 1.0, "center"},
		{0.9, 0, 0, 1.0, "inside on x (0.9 < 1.0)"},
		{1.0, 0, 0, 0.0, "on x boundary (1.0 not < 1.0)"},
		{0, 0.4, 0, 1.0, "inside on y (0.4 < 0.5)"},
		{0, 0.5, 0, 0.0, "on y boundary (0.5 not < 0.5)"},
		{0, 0, 0.24, 1.0, "inside on z (0.24 < 0.25)"},
		{0, 0, 0.25, 0.0, "on z boundary (0.25 not < 0.25)"},
	}
	for _, c := range cases {
		got := b.Density(c.x, c.y, c.z)
		if got != c.want {
			t.Errorf("Box.Density(%v,%v,%v) [%s] = %v, want %v", c.x, c.y, c.z, c.label, got, c.want)
		}
	}
}

func TestCylinderGeometry(t *testing.T) {
	// Cylinder along Z axis, P0=(0,0,-1), P1=(0,0,1), radius=0.5, rho=1.0
	cyl := &Cylinder{P0: mgl64.Vec3{0, 0, -1}, P1: mgl64.Vec3{0, 0, 1}, Radius: 0.5, Rho: 1.0}

	cases := []struct {
		x, y, z float64
		want    float64
		label   string
	}{
		{0, 0, 0, 1.0, "on axis at mid"},
		{0.4, 0, 0, 1.0, "within radius at mid"},
		{0.5, 0, 0, 0.0, "exactly at radius (strict <)"},
		{0, 0, -1.0, 1.0, "at P0 cap face (c=0, d=0 < r)"},
		{0, 0, 1.0, 1.0, "at P1 cap face (c=1, d=0 < r)"},
		{0, 0, -1.001, 0.0, "just past P0 cap (c<0)"},
		{0, 0, 1.001, 0.0, "just past P1 cap (c>1)"},
		{0.4, 0, 1.0, 1.0, "inside radius at P1 face"},
	}
	for _, c := range cases {
		got := cyl.Density(c.x, c.y, c.z)
		if got != c.want {
			t.Errorf("Cylinder.Density(%v,%v,%v) [%s] = %v, want %v", c.x, c.y, c.z, c.label, got, c.want)
		}
	}
}

func TestObjectCollectionClamping(t *testing.T) {
	// Two overlapping spheres each with rho=0.8 — sum 1.6, should clamp to 1.0
	s1 := &Sphere{Center: mgl64.Vec3{0, 0, 0}, Radius: 0.5, Rho: 0.8}
	s2 := &Sphere{Center: mgl64.Vec3{0, 0, 0}, Radius: 0.5, Rho: 0.8}
	coll := &ObjectCollection{Objects: []Object{s1, s2}}

	got := coll.Density(0, 0, 0)
	if got != 1.0 {
		t.Errorf("overlapping spheres density = %v, want 1.0 (clamped)", got)
	}

	// GreedyDensEval should short-circuit and return first hit
	coll.GreedyDensEval = true
	got = coll.Density(0, 0, 0)
	if got != 0.8 {
		t.Errorf("greedy eval: density = %v, want 0.8 (first sphere only)", got)
	}
}

// ── VoxelGrid coordinate layout ───────────────────────────────────────────────

// TestVoxelGridCoordinateLayout verifies that Density() reads Rho using the
// [z][x][y] flat layout (index = z*NX*NY + x*NY + y).
// If x and y were swapped in the index formula, the sub-tests would fail.
func TestVoxelGridCoordinateLayout(t *testing.T) {
	const N = 3
	rho := make([]float64, N*N*N)

	// Mark (z=0, x=2, y=0): index = 0*9 + 2*3 + 0 = 6
	// World: x=+1 (max), y=-1 (min), z=-1 (min)
	rho[0*N*N+2*N+0] = 1.0
	vg := &VoxelGrid{Rho: rho, NX: N, NY: N, NZ: N}

	if got := vg.Density(1.0, -1.0, -1.0); math.Abs(got-1.0) > 1e-9 {
		t.Errorf("Density(x=1,y=-1,z=-1) = %.4f, want 1.0 — layout may be wrong", got)
	}
	// x/y-swapped position should be empty
	if got := vg.Density(-1.0, 1.0, -1.0); got > 1e-9 {
		t.Errorf("Density(x=-1,y=1,z=-1) = %.4f, want ~0 — x/y may be swapped", got)
	}

	// Test with non-zero z: mark (z=1, x=0, y=0): index = 1*9 + 0*3 + 0 = 9
	// World: x=-1 (min), y=-1 (min), z=0 (mid)
	rho2 := make([]float64, N*N*N)
	rho2[1*N*N+0*N+0] = 1.0
	vg2 := &VoxelGrid{Rho: rho2, NX: N, NY: N, NZ: N}
	if got := vg2.Density(-1.0, -1.0, 0.0); math.Abs(got-1.0) > 1e-9 {
		t.Errorf("Density(x=-1,y=-1,z=0) = %.4f, want 1.0", got)
	}
	if got := vg2.Density(-1.0, 0.0, -1.0); got > 1e-9 {
		t.Errorf("Density(x=-1,y=0,z=-1) = %.4f, want ~0 — z/y may be swapped", got)
	}
}

// ── VoxelGrid round-trip ──────────────────────────────────────────────────────

// TestVoxelGridRoundTrip verifies that ExportToRaw + VoxelGridFromRaw preserves
// spatial positions for an asymmetric two-sphere scene:
//   - large sphere at (0.5, 0, 0) — offset in x
//   - small sphere at (0, 0.3, 0) — offset in y
//
// If x/y were swapped the density checks at the swapped positions would fail.
func TestVoxelGridRoundTrip(t *testing.T) {
	const res = 64

	largeSphere := &Sphere{Center: mgl64.Vec3{0.5, 0.0, 0.0}, Radius: 0.15, Rho: 1.0}
	smallSphere := &Sphere{Center: mgl64.Vec3{0.0, 0.3, 0.0}, Radius: 0.08, Rho: 1.0}

	rho := make([]float64, res*res*res)
	for k := 0; k < res; k++ {
		for i := 0; i < res; i++ {
			for j := 0; j < res; j++ {
				x := float64(i)/float64(res)*2.0 - 1.0
				y := float64(j)/float64(res)*2.0 - 1.0
				z := float64(k)/float64(res)*2.0 - 1.0
				d := largeSphere.Density(x, y, z) + smallSphere.Density(x, y, z)
				if d > 1 {
					d = 1
				}
				rho[k*res*res+i*res+j] = d
			}
		}
	}
	original := &VoxelGrid{Rho: rho, NX: res, NY: res, NZ: res}

	tmp := filepath.Join(t.TempDir(), "volume.raw")
	if err := original.ExportToRaw(tmp, res); err != nil {
		t.Fatalf("ExportToRaw: %v", err)
	}
	reloaded, err := VoxelGridFromRaw(tmp, [3]int{res, res, res}, "uint8")
	if err != nil {
		t.Fatalf("VoxelGridFromRaw: %v", err)
	}
	defer os.Remove(tmp)

	// large sphere at (0.5, 0, 0)
	if got := reloaded.Density(0.5, 0.0, 0.0); got < 0.5 {
		t.Errorf("large sphere centre: Density(0.5,0,0)=%.3f want >0.5", got)
	}
	if got := reloaded.Density(0.0, 0.5, 0.0); got > 0.1 {
		t.Errorf("x/y-swapped large sphere: Density(0,0.5,0)=%.3f want ~0 (x/y swapped?)", got)
	}

	// small sphere at (0, 0.3, 0)
	if got := reloaded.Density(0.0, 0.3, 0.0); got < 0.5 {
		t.Errorf("small sphere centre: Density(0,0.3,0)=%.3f want >0.5", got)
	}
	if got := reloaded.Density(0.3, 0.0, 0.0); got > 0.1 {
		t.Errorf("x/y-swapped small sphere: Density(0.3,0,0)=%.3f want ~0 (x/y swapped?)", got)
	}
}

// ── VoxelGrid edge cases ──────────────────────────────────────────────────────

func TestVoxelGridOutsideBounds(t *testing.T) {
	rho := make([]float64, 8) // 2×2×2 all zeros
	for i := range rho {
		rho[i] = 1.0
	}
	vg := &VoxelGrid{Rho: rho, NX: 2, NY: 2, NZ: 2}

	cases := []struct{ x, y, z float64 }{
		{1.001, 0, 0},
		{-1.001, 0, 0},
		{0, 1.001, 0},
		{0, -1.001, 0},
		{0, 0, 1.001},
		{0, 0, -1.001},
		{2, 2, 2},
	}
	for _, c := range cases {
		if got := vg.Density(c.x, c.y, c.z); got != 0.0 {
			t.Errorf("Density(%v,%v,%v) = %v, want 0.0 (outside bounds)", c.x, c.y, c.z, got)
		}
	}

	// Exactly at boundary corners should NOT be clipped
	if got := vg.Density(1.0, 1.0, 1.0); got != 1.0 {
		t.Errorf("Density(1,1,1) = %v, want 1.0 (corner is valid)", got)
	}
	if got := vg.Density(-1.0, -1.0, -1.0); got != 1.0 {
		t.Errorf("Density(-1,-1,-1) = %v, want 1.0 (corner is valid)", got)
	}
}

func TestVoxelGridTrilinearInterpolation(t *testing.T) {
	// 2×2×2 grid, all ones — interpolation must return 1 everywhere inside
	rho := make([]float64, 8)
	for i := range rho {
		rho[i] = 1.0
	}
	vg := &VoxelGrid{Rho: rho, NX: 2, NY: 2, NZ: 2}

	for _, pos := range [][3]float64{{0, 0, 0}, {0.5, -0.5, 0.3}, {1, 1, 1}, {-1, -1, -1}} {
		if got := vg.Density(pos[0], pos[1], pos[2]); math.Abs(got-1.0) > 1e-9 {
			t.Errorf("uniform-1 grid: Density(%v,%v,%v)=%v, want 1.0", pos[0], pos[1], pos[2], got)
		}
	}

	// 2×2×2 grid with only index-0 voxel set (z=0,x=0,y=0 → world (-1,-1,-1))
	// At the exact corner, wx=wy=wz=0 so result is Rho[0]=1.
	// At world (0,-1,-1): x maps to voxel 0.5 → x0=0,wx=0.5 → lerp(1.0, 0.0, 0.5)=0.5
	rho2 := make([]float64, 8)
	rho2[0] = 1.0 // index 0 = z=0,x=0,y=0
	vg2 := &VoxelGrid{Rho: rho2, NX: 2, NY: 2, NZ: 2}

	if got := vg2.Density(-1.0, -1.0, -1.0); math.Abs(got-1.0) > 1e-9 {
		t.Errorf("single-voxel corner: Density(-1,-1,-1)=%v, want 1.0", got)
	}
	// halfway in x from corner: wx=0.5, result should be 0.5
	if got := vg2.Density(0.0, -1.0, -1.0); math.Abs(got-0.5) > 1e-9 {
		t.Errorf("single-voxel halfway-x: Density(0,-1,-1)=%v, want 0.5", got)
	}
}

func TestVoxelGridExportToRaw_AllZero(t *testing.T) {
	// All-zero grid: ExportToRaw should not panic (divide-by-zero in normalization)
	rho := make([]float64, 27) // 3×3×3 all zero
	vg := &VoxelGrid{Rho: rho, NX: 3, NY: 3, NZ: 3}
	tmp := filepath.Join(t.TempDir(), "zero.raw")

	// Should not panic
	if err := vg.ExportToRaw(tmp, 3); err != nil {
		t.Fatalf("ExportToRaw on all-zero grid returned error: %v", err)
	}
}

func TestVoxelGridFromRaw_Uint8Precision(t *testing.T) {
	// Write known uint8 bytes, reload, verify values
	tmp := filepath.Join(t.TempDir(), "u8.raw")
	data := []byte{255, 128, 0}
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	vg, err := VoxelGridFromRaw(tmp, [3]int{1, 1, 3}, "uint8")
	if err != nil {
		t.Fatalf("VoxelGridFromRaw: %v", err)
	}
	want := []float64{1.0, 128.0 / 255.0, 0.0}
	for i, w := range want {
		if math.Abs(vg.Rho[i]-w) > 1e-10 {
			t.Errorf("Rho[%d] = %.6f, want %.6f", i, vg.Rho[i], w)
		}
	}
}

// ── Non-cubic VoxelGrid coordinate correctness ────────────────────────────────

// TestVoxelGridNonCubic verifies that Density() correctly maps world coordinates
// to voxel indices when NX ≠ NY ≠ NZ (non-cubic volumes).
//
// Background: the CUDA texture path sets extent.width=NY, extent.height=NX and
// calls tex3D(tex, (worldY+1)*0.5, (worldX+1)*0.5, ...) — swapping x and y in
// the tex3D call to compensate for the swapped extent. This test validates the
// equivalent CPU indexing formula so that any future changes to the layout are
// caught here first.
func TestVoxelGridNonCubic(t *testing.T) {
	// Volume with NX=2, NY=4, NZ=3 — all dimensions differ.
	// Flat layout: index = k*NX*NY + i*NY + j  (k=z, i=x, j=y)
	nx, ny, nz := 2, 4, 3
	rho := make([]float64, nx*ny*nz)

	// Place a marker at (x=1, y=3, z=2) — maximum corner in each dimension.
	// Flat index: k*nx*ny + i*ny + j = 2*2*4 + 1*4 + 3 = 16+4+3 = 23
	markerI, markerJ, markerK := 1, 3, 2
	rho[markerK*nx*ny+markerI*ny+markerJ] = 1.0
	vg := &VoxelGrid{Rho: rho, NX: nx, NY: ny, NZ: nz}

	// World coordinates for maximum corner: x→1, y→1, z→1
	if got := vg.Density(1.0, 1.0, 1.0); math.Abs(got-1.0) > 1e-9 {
		t.Errorf("non-cubic max corner: Density(1,1,1)=%.4f want 1.0", got)
	}
	// Swapping x and y should give zero (confirms axes are not mixed up)
	if got := vg.Density(-1.0, 1.0, 1.0); got > 1e-9 {
		t.Errorf("non-cubic x/y swap check: Density(-1,1,1)=%.4f want ~0 (x/y mixed?)", got)
	}
	if got := vg.Density(1.0, -1.0, 1.0); got > 1e-9 {
		t.Errorf("non-cubic x/y swap check: Density(1,-1,1)=%.4f want ~0 (x/y mixed?)", got)
	}

	// Interior asymmetric marker: (i=0, j=1, k=0) in NX=2, NY=4, NZ=3 grid.
	// Density() uses (N-1) scaling: worldY for j=1 is 1/(NY-1)*2-1 = 1/3*2-1 = -1/3.
	rho2 := make([]float64, nx*ny*nz)
	rho2[0*nx*ny+0*ny+1] = 1.0 // flat index 1: k=0, i=0, j=1
	vg2 := &VoxelGrid{Rho: rho2, NX: nx, NY: ny, NZ: nz}

	worldYj1 := float64(1)/float64(ny-1)*2.0 - 1.0 // j=1 in NY=4 grid: -1/3
	if got := vg2.Density(-1.0, worldYj1, -1.0); math.Abs(got-1.0) > 1e-9 {
		t.Errorf("non-cubic interior: Density(-1,%.4f,-1)=%.4f want 1.0", worldYj1, got)
	}
	// Swapping worldX and worldY must give a different (zero) position.
	if got := vg2.Density(worldYj1, -1.0, -1.0); got > 1e-9 {
		t.Errorf("non-cubic x/y swap: Density(%.4f,-1,-1)=%.4f want ~0", worldYj1, got)
	}
}

// ── TessellatedObjColl seam tests ────────────────────────────────────────────

// TestTessellatedDensityXAxisCylinder checks that a cylinder spanning the full
// width of a unit cell ([0,1] in x) tiles seamlessly: density on the cylinder
// axis must be non-zero and equal on both sides of every UC boundary.
func TestTessellatedDensityXAxisCylinder(t *testing.T) {
	cyl := &Cylinder{P0: mgl64.Vec3{0, 0.5, 0.5}, P1: mgl64.Vec3{1, 0.5, 0.5}, Radius: 0.1, Rho: 1.0}
	uc := UnitCell{
		Objects:                            ObjectCollection{Objects: []Object{cyl}},
		Xmin: 0, Xmax: 1, Ymin: 0, Ymax: 1, Zmin: 0, Zmax: 1,
	}
	lat := &TessellatedObjColl{UC: uc, Xmin: -2, Xmax: 2, Ymin: -2, Ymax: 2, Zmin: -2, Zmax: 2}

	// Sample along the cylinder axis at x = ..., -1, -0.5, 0, 0.5, 1, 1.5, ...
	// All should return Rho=1 (d=0 from axis, c in [0,1] in the appropriate periodic copy).
	xs := []float64{-1.9, -1.5, -1.0, -0.5, 0.0, 0.5, 1.0, 1.5, 1.9}
	for _, x := range xs {
		got := lat.Density(x, 0.5, 0.5)
		if got == 0 {
			t.Errorf("x=%.1f: density=0 (seam on cylinder axis)", x)
		}
	}

	// Verify continuity across x=1.0 (a UC boundary): just inside vs just outside.
	eps := 1e-9
	dIn := lat.Density(1.0-eps, 0.5, 0.5)
	dOut := lat.Density(1.0+eps, 0.5, 0.5)
	if dIn == 0 || dOut == 0 {
		t.Errorf("seam at x=1: density %.3f (inside) %.3f (outside) should both be non-zero", dIn, dOut)
	}
}

// TestTessellatedDensityKelvinZBoundary checks that the Kelvin unit cell tiles
// correctly across its z faces. Struts (0.5,0,0.75)→(0.5,0.25,1.0) and
// (0.5,0,0.25)→(0.5,0.25,0.0) meet at the z=0/1 face: density must be
// non-zero on both sides of z=1 for a point on those struts.
func TestTessellatedDensityKelvinZBoundary(t *testing.T) {
	uc := MakeKelvin(0.1, 1.0)
	lat := &TessellatedObjColl{UC: uc, Xmin: -2, Xmax: 2, Ymin: -2, Ymax: 2, Zmin: -2, Zmax: 2}

	// Point on strut (0.5,0,0.75)→(0.5,0.25,1.0) at parameter c≈0.75:
	// real-space position (0.5, 0.1875, 0.9375), well inside radius 0.1.
	// Its periodic image across z=1 is (0.5, 0.1875, 1.0625) which folds to
	// (0.5, 0.1875, 0.0625) — on the mirror strut (0.5,0,0.25)→(0.5,0.25,0.0).
	insideZ := lat.Density(0.5, 0.1875, 0.9375)
	if insideZ == 0 {
		t.Error("Kelvin z-boundary: density=0 just inside z=1 (on strut axis)")
	}
	outsideZ := lat.Density(0.5, 0.1875, 1.0625)
	if outsideZ == 0 {
		t.Error("Kelvin z-boundary: density=0 just outside z=1 (should fold to mirror strut)")
	}

	// Density values should be equal (both points are symmetric with c=0.75 on respective struts).
	if math.Abs(insideZ-outsideZ) > 1e-9 {
		t.Errorf("Kelvin z-boundary density mismatch: inside=%.4f outside=%.4f", insideZ, outsideZ)
	}
}

// TestTessellatedDensityKelvinYFaceStruts checks struts that lie entirely in a
// face of the Kelvin unit cell (y=0 and y=1). These struts must contribute
// density on both sides of the y=0 face.
func TestTessellatedDensityKelvinYFaceStruts(t *testing.T) {
	uc := MakeKelvin(0.1, 1.0)
	lat := &TessellatedObjColl{UC: uc, Xmin: -2, Xmax: 2, Ymin: -2, Ymax: 2, Zmin: -2, Zmax: 2}

	// Strut (0.25,0,0.5)→(0.5,0,0.75) lies on y=0. Midpoint=(0.375, 0, 0.625), d_perp=0.
	// A point offset by (0, ±eps, 0) from the midpoint must be inside radius 0.1.
	eps := 0.01
	dAbove := lat.Density(0.375, eps, 0.625)
	dBelow := lat.Density(0.375, -eps, 0.625) // folds to y=1-eps, hits y=1 mirror strut
	if dAbove == 0 {
		t.Errorf("Kelvin y-face strut: density=0 at y=+%.3f (just above y=0 face)", eps)
	}
	if dBelow == 0 {
		t.Errorf("Kelvin y-face strut: density=0 at y=-%.3f (just below y=0 face, should fold to y=1 mirror)", eps)
	}
}

// TestVoxelGridNonCubicAxisSeparation tests with a larger asymmetric non-cubic
// grid that density placed at an x-offset is not visible at the equivalent y-offset.
// Uses (N-1) world-coordinate convention to match Density().
func TestVoxelGridNonCubicAxisSeparation(t *testing.T) {
	// NX=16, NY=32, NZ=16: voxel step ~0.133 in all dims, large enough for radius-0.3 sphere.
	// Sphere is placed at x=0.5 ONLY — nothing at y=0.5.
	// If x and y axes were swapped, the sphere would appear at (0, 0.5, 0) instead of (0.5, 0, 0).
	nx, ny, nz := 16, 32, 16
	sphere := &Sphere{Center: mgl64.Vec3{0.5, 0.0, 0.0}, Radius: 0.3, Rho: 1.0}

	rho := make([]float64, nx*ny*nz)
	for k := 0; k < nz; k++ {
		for i := 0; i < nx; i++ {
			for j := 0; j < ny; j++ {
				x := float64(i) / float64(nx-1) * 2.0 - 1.0
				y := float64(j) / float64(ny-1) * 2.0 - 1.0
				z := float64(k) / float64(nz-1) * 2.0 - 1.0
				rho[k*nx*ny+i*ny+j] = sphere.Density(x, y, z)
			}
		}
	}
	vg := &VoxelGrid{Rho: rho, NX: nx, NY: ny, NZ: nz}

	// Sphere centre at x=0.5 must register high density.
	if got := vg.Density(0.5, 0.0, 0.0); got < 0.8 {
		t.Errorf("sphere at x=0.5: Density(0.5,0,0)=%.3f want >0.8", got)
	}
	// Nothing is at y=0.5 — if axes were swapped the sphere would appear here.
	if got := vg.Density(0.0, 0.5, 0.0); got > 0.1 {
		t.Errorf("no sphere at y=0.5: Density(0,0.5,0)=%.3f want ~0 (x/y axis swap?)", got)
	}
}
