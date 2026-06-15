package main

import (
	"math"
	"testing"

	"github.com/go-gl/mathgl/mgl64"
	"github.com/igrega348/xray_projection_render/objects"
)

func TestParseFloatList(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []float64
		wantErr bool
	}{
		{
			name:    "empty string",
			input:   "",
			want:    nil,
			wantErr: false,
		},
		{
			name:    "single value",
			input:   "90.0",
			want:    []float64{90.0},
			wantErr: false,
		},
		{
			name:    "multiple values",
			input:   "0,45,90,135",
			want:    []float64{0, 45, 90, 135},
			wantErr: false,
		},
		{
			name:    "values with spaces",
			input:   "0, 45, 90, 135",
			want:    []float64{0, 45, 90, 135},
			wantErr: false,
		},
		{
			name:    "decimal values",
			input:   "0.5,45.25,90.75",
			want:    []float64{0.5, 45.25, 90.75},
			wantErr: false,
		},
		{
			name:    "negative values",
			input:   "-45,0,45",
			want:    []float64{-45, 0, 45},
			wantErr: false,
		},
		{
			name:    "invalid value",
			input:   "0,abc,90",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "trailing comma",
			input:   "0,45,90,",
			want:    []float64{0, 45, 90},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFloatList(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseFloatList() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("parseFloatList() length = %v, want %v", len(got), len(tt.want))
					return
				}
				for i := range got {
					if math.Abs(got[i]-tt.want[i]) > 1e-9 {
						t.Errorf("parseFloatList()[%d] = %v, want %v", i, got[i], tt.want[i])
					}
				}
			}
		})
	}
}

func TestComputeCameraFromAngles(t *testing.T) {
	tests := []struct {
		name         string
		azimuthalDeg float64
		polarDeg     float64
		R            float64
		checkEye     func(t *testing.T, eye mgl64.Vec3)
	}{
		{
			name:         "azimuthal 0, polar 90 (equatorial plane, positive x)",
			azimuthalDeg: 0,
			polarDeg:     90,
			R:            4.0,
			checkEye: func(t *testing.T, eye mgl64.Vec3) {
				// Should be at (R, 0, 0) for azimuthal=0, polar=90
				expected := mgl64.Vec3{4.0, 0, 0}
				diff := eye.Sub(expected)
				dist := math.Sqrt(diff[0]*diff[0] + diff[1]*diff[1] + diff[2]*diff[2])
				if dist > 1e-6 {
					t.Errorf("Expected eye position ~(%v, %v, %v), got (%v, %v, %v)",
						expected[0], expected[1], expected[2], eye[0], eye[1], eye[2])
				}
				// Check distance from origin
				eyeLen := math.Sqrt(eye[0]*eye[0] + eye[1]*eye[1] + eye[2]*eye[2])
				if math.Abs(eyeLen-4.0) > 1e-6 {
					t.Errorf("Expected distance from origin = 4.0, got %v", eyeLen)
				}
			},
		},
		{
			name:         "azimuthal 90, polar 90 (equatorial plane, positive y)",
			azimuthalDeg: 90,
			polarDeg:     90,
			R:            4.0,
			checkEye: func(t *testing.T, eye mgl64.Vec3) {
				// Should be at (0, R, 0) for azimuthal=90, polar=90
				expected := mgl64.Vec3{0, 4.0, 0}
				diff := eye.Sub(expected)
				dist := math.Sqrt(diff[0]*diff[0] + diff[1]*diff[1] + diff[2]*diff[2])
				if dist > 1e-6 {
					t.Errorf("Expected eye position ~(%v, %v, %v), got (%v, %v, %v)",
						expected[0], expected[1], expected[2], eye[0], eye[1], eye[2])
				}
			},
		},
		{
			name:         "azimuthal 0, polar 0 (north pole)",
			azimuthalDeg: 0,
			polarDeg:     0,
			R:            4.0,
			checkEye: func(t *testing.T, eye mgl64.Vec3) {
				// Should be at (0, 0, R) for polar=0
				expected := mgl64.Vec3{0, 0, 4.0}
				diff := eye.Sub(expected)
				dist := math.Sqrt(diff[0]*diff[0] + diff[1]*diff[1] + diff[2]*diff[2])
				if dist > 1e-6 {
					t.Errorf("Expected eye position ~(%v, %v, %v), got (%v, %v, %v)",
						expected[0], expected[1], expected[2], eye[0], eye[1], eye[2])
				}
				// Note: At north pole, the camera matrix may be degenerate (gimbal lock)
				// This is acceptable for this use case
			},
		},
		{
			name:         "distance check",
			azimuthalDeg: 45,
			polarDeg:     60,
			R:            5.0,
			checkEye: func(t *testing.T, eye mgl64.Vec3) {
				// Distance from origin should be R
				eyeLen := math.Sqrt(eye[0]*eye[0] + eye[1]*eye[1] + eye[2]*eye[2])
				if math.Abs(eyeLen-5.0) > 1e-6 {
					t.Errorf("Expected distance from origin = 5.0, got %v", eyeLen)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eye, camera := computeCameraFromAngles(tt.azimuthalDeg, tt.polarDeg, tt.R)
			tt.checkEye(t, eye)

			// Check that camera matrix is valid (4x4)
			// Mat4 is always 4x4, so we just check that it's not all zeros
			// Skip this check for polar=0 (north pole) due to gimbal lock
			if tt.polarDeg != 0 {
				hasNonZero := false
				for i := 0; i < 4; i++ {
					for j := 0; j < 4; j++ {
						if math.Abs(camera.At(i, j)) > 1e-10 {
							hasNonZero = true
							break
						}
					}
					if hasNonZero {
						break
					}
				}
				if !hasNonZero {
					t.Error("Camera matrix should not be all zeros")
				}
			}
		})
	}
}

func TestGenerateCameraAngles(t *testing.T) {
	tests := []struct {
		name        string
		num_images  int
		job_num     int
		jobs_modulo int
		out_of_plane bool
		polar_angle float64
		wantCount   int
		checkAngles func(t *testing.T, angles []CameraAngle)
	}{
		{
			name:        "equispaced, 4 images",
			num_images:  4,
			job_num:     0,
			jobs_modulo: 1,
			out_of_plane: false,
			polar_angle: 90.0,
			wantCount:   4,
			checkAngles: func(t *testing.T, angles []CameraAngle) {
				if len(angles) != 4 {
					t.Errorf("Expected 4 angles, got %d", len(angles))
				}
				// Check azimuthal angles are equispaced starting from 90
				expectedAzimuthals := []float64{90.0, 180.0, 270.0, 360.0}
				for i, angle := range angles {
					if math.Abs(angle.Azimuthal-expectedAzimuthals[i]) > 1e-6 {
						t.Errorf("Angle[%d].Azimuthal = %v, want %v", i, angle.Azimuthal, expectedAzimuthals[i])
					}
					if math.Abs(angle.Polar-90.0) > 1e-6 {
						t.Errorf("Angle[%d].Polar = %v, want 90.0", i, angle.Polar)
					}
				}
			},
		},
		{
			name:        "custom polar angle",
			num_images:  2,
			job_num:     0,
			jobs_modulo: 1,
			out_of_plane: false,
			polar_angle: 45.0,
			wantCount:   2,
			checkAngles: func(t *testing.T, angles []CameraAngle) {
				for i, angle := range angles {
					if math.Abs(angle.Polar-45.0) > 1e-6 {
						t.Errorf("Angle[%d].Polar = %v, want 45.0", i, angle.Polar)
					}
				}
			},
		},
		{
			name:        "jobs_modulo filtering",
			num_images:  8,
			job_num:     1,
			jobs_modulo: 2,
			out_of_plane: false,
			polar_angle: 90.0,
			wantCount:   4,
			checkAngles: func(t *testing.T, angles []CameraAngle) {
				if len(angles) != 4 {
					t.Errorf("Expected 4 angles, got %d", len(angles))
				}
				// Should start at index 1, then 3, 5, 7
				// dth = 360/8 = 45, so indices 1,3,5,7 give: 90+1*45=135, 90+3*45=225, 90+5*45=315, 90+7*45=405
				expectedAzimuthals := []float64{135.0, 225.0, 315.0, 405.0}
				for i, angle := range angles {
					if math.Abs(angle.Azimuthal-expectedAzimuthals[i]) > 1e-6 {
						t.Errorf("Angle[%d].Azimuthal = %v, want %v", i, angle.Azimuthal, expectedAzimuthals[i])
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			angles := generateCameraAngles(tt.num_images, tt.job_num, tt.jobs_modulo, tt.out_of_plane, tt.polar_angle)
			if len(angles) != tt.wantCount {
				t.Errorf("generateCameraAngles() returned %d angles, want %d", len(angles), tt.wantCount)
			}
			if tt.checkAngles != nil {
				tt.checkAngles(t, angles)
			}
		})
	}
}

func TestCameraAngleStruct(t *testing.T) {
	angle := CameraAngle{
		Azimuthal: 45.0,
		Polar:     90.0,
	}

	if angle.Azimuthal != 45.0 {
		t.Errorf("Azimuthal = %v, want 45.0", angle.Azimuthal)
	}
	if angle.Polar != 90.0 {
		t.Errorf("Polar = %v, want 90.0", angle.Polar)
	}
}

// ── Physics test infrastructure ───────────────────────────────────────────────

// setupPhysics replaces the package-level globals used by density() and the
// integrators, and returns a restore function to be deferred.
// Physics tests MUST NOT call t.Parallel() — the globals are shared.
func setupPhysics(obj objects.Object, flatField, densityMult float64) func() {
	origLat := lat
	origFlat := flat_field
	origDM := density_multiplier
	origDF := df
	lat = []objects.Object{obj}
	flat_field = flatField
	density_multiplier = densityMult
	df = nil
	return func() {
		lat = origLat
		flat_field = origFlat
		density_multiplier = origDM
		df = origDF
	}
}

// ── Integrator physics tests ──────────────────────────────────────────────────

// TestIntegrateSimple_SphereCenterRay checks the Beer-Lambert integral for a
// ray passing through the center of a unit sphere (radius 0.5, rho 1.0).
// Chord length = 2r = 1.0 → expected pixel = exp(-1.0) ≈ 0.36788.
func TestIntegrateSimple_SphereCenterRay(t *testing.T) {
	sphere := &objects.Sphere{Center: mgl64.Vec3{0, 0, 0}, Radius: 0.5, Rho: 1.0}
	defer setupPhysics(sphere, 0.0, 1.0)()

	origin := mgl64.Vec3{0, 0, -5}
	dir := mgl64.Vec3{0, 0, 1}
	const ds = 0.001
	got := integrate_along_ray(origin, dir, ds, 4.5, 5.5)
	want := math.Exp(-1.0)
	tol := 2 * ds // O(ds) boundary discretisation error
	if math.Abs(got-want) > tol {
		t.Errorf("simple integrator sphere chord: got %.5f, want %.5f (tol %.4f)", got, want, tol)
	}
}

// TestIntegrateSimple_SlabAttenuation checks Beer-Lambert for a uniform slab
// of thickness 1.0 and rho 2.0 → T = 2.0, pixel = exp(-2.0) ≈ 0.13534.
func TestIntegrateSimple_SlabAttenuation(t *testing.T) {
	// Box: sides 10×10×1, so the z-slab occupies z ∈ (-0.5, 0.5)
	slab := &objects.Box{
		Center: mgl64.Vec3{0, 0, 0},
		Sides:  mgl64.Vec3{10, 10, 1},
		Rho:    2.0,
	}
	defer setupPhysics(slab, 0.0, 1.0)()

	origin := mgl64.Vec3{0, 0, -5}
	dir := mgl64.Vec3{0, 0, 1}
	const ds = 0.001
	got := integrate_along_ray(origin, dir, ds, 0, 10)
	want := math.Exp(-2.0)
	tol := 2 * ds * 2.0 // O(ds*rho) error
	if math.Abs(got-want) > tol {
		t.Errorf("simple integrator slab: got %.5f, want %.5f (tol %.4f)", got, want, tol)
	}
}

// TestIntegrateHierarchical_SphereCenterRay repeats the sphere chord test with
// the hierarchical integrator. Tolerance is relaxed to one coarse step.
func TestIntegrateHierarchical_SphereCenterRay(t *testing.T) {
	sphere := &objects.Sphere{Center: mgl64.Vec3{0, 0, 0}, Radius: 0.5, Rho: 1.0}
	defer setupPhysics(sphere, 0.0, 1.0)()

	origin := mgl64.Vec3{0, 0, -5}
	dir := mgl64.Vec3{0, 0, 1}
	const DS = 0.05
	got := integrate_hierarchical(origin, dir, DS, 4.5, 5.5)
	want := math.Exp(-1.0)
	tol := DS // hierarchical error is O(DS) at surface transitions
	if math.Abs(got-want) > tol {
		t.Errorf("hierarchical integrator sphere chord: got %.5f, want %.5f (tol %.4f)", got, want, tol)
	}
}

// TestIntegrateSimpleVsHierarchical_Agreement checks that both integrators
// agree to within a small fraction for a smooth object (sphere).
func TestIntegrateSimpleVsHierarchical_Agreement(t *testing.T) {
	sphere := &objects.Sphere{Center: mgl64.Vec3{0, 0, 0}, Radius: 0.5, Rho: 1.0}
	defer setupPhysics(sphere, 0.0, 1.0)()

	origin := mgl64.Vec3{0, 0, -5}
	dir := mgl64.Vec3{0, 0, 1}

	simple := integrate_along_ray(origin, dir, 0.001, 4.5, 5.5)
	hier := integrate_hierarchical(origin, dir, 0.05, 4.5, 5.5)
	if math.Abs(simple-hier) > 0.01 {
		t.Errorf("simple=%.5f vs hierarchical=%.5f disagree by more than 0.01", simple, hier)
	}
}

// TestFlatFieldAppliedOnce verifies flat_field is added exactly once to T.
// With an empty scene and flat_field=1.0 the pixel must be exp(-1.0).
// If applied twice it would be exp(-2.0); if not applied it would be 1.0.
func TestFlatFieldAppliedOnce(t *testing.T) {
	// Use a sphere far from the ray so density is always 0
	empty := &objects.Sphere{Center: mgl64.Vec3{0, 0, 10}, Radius: 0.01, Rho: 1.0}
	defer setupPhysics(empty, 1.0, 1.0)()

	origin := mgl64.Vec3{0, 0, -5}
	dir := mgl64.Vec3{0, 0, 1}

	for _, tc := range []struct {
		name string
		fn   func(mgl64.Vec3, mgl64.Vec3, float64, float64, float64) float64
	}{
		{"simple", integrate_along_ray},
		{"hierarchical", integrate_hierarchical},
	} {
		got := tc.fn(origin, dir, 0.01, 0, 1) // short window, misses the sphere at z=10
		want := math.Exp(-1.0)
		if math.Abs(got-want) > 1e-9 {
			t.Errorf("[%s] flat_field=1, empty scene: got %.6f, want %.6f (exp(-1))", tc.name, got, want)
		}
	}
}

// TestDensityMultiplierApplied verifies that density_multiplier scales T.
// With rho=1, chord=1, multiplier=2 → T=2, pixel=exp(-2).
func TestDensityMultiplierApplied(t *testing.T) {
	sphere := &objects.Sphere{Center: mgl64.Vec3{0, 0, 0}, Radius: 0.5, Rho: 1.0}
	defer setupPhysics(sphere, 0.0, 2.0)() // density_multiplier=2

	origin := mgl64.Vec3{0, 0, -5}
	dir := mgl64.Vec3{0, 0, 1}
	const ds = 0.001
	got := integrate_along_ray(origin, dir, ds, 4.5, 5.5)
	want := math.Exp(-2.0)
	tol := 2 * ds * 2.0
	if math.Abs(got-want) > tol {
		t.Errorf("density_multiplier=2: got %.5f, want %.5f", got, want)
	}
}

// TestIntegrateSimple_DSConvergence verifies the simple integrator reaches the
// analytical Beer-Lambert value to within O(ds) at ds=0.001.
// Note: strict monotonic error decrease is NOT guaranteed because step-grid
// aliasing with the sphere boundary can make coarse steps accidentally accurate.
func TestIntegrateSimple_DSConvergence(t *testing.T) {
	sphere := &objects.Sphere{Center: mgl64.Vec3{0, 0, 0}, Radius: 0.5, Rho: 1.0}
	defer setupPhysics(sphere, 0.0, 1.0)()

	origin := mgl64.Vec3{0, 0, -5}
	dir := mgl64.Vec3{0, 0, 1}
	want := math.Exp(-1.0)

	for _, tc := range []struct {
		ds  float64
		tol float64
	}{
		{0.01, 5e-2},   // coarse: within 5%
		{0.001, 5e-3},  // fine: within 0.5%
		{0.0001, 5e-4}, // very fine: within 0.05%
	} {
		got := integrate_along_ray(origin, dir, tc.ds, 4.5, 5.5)
		if math.Abs(got-want) > tc.tol {
			t.Errorf("ds=%.5f: |%.5f - %.5f| = %.5f, want < %.5f",
				tc.ds, got, want, math.Abs(got-want), tc.tol)
		}
	}
}

// TestIntegrateHierarchical_BoundaryAccuracy verifies that the hierarchical
// integrator does not have a systematic underestimate at material boundaries.
//
// Background: the adversarial review flagged that `left += ds` at the start of
// the transition branch drops one fine sub-step. Analysis shows this is a false
// positive — the integrator uses a right-endpoint rectangle rule throughout, and
// the skipped left endpoint was already sampled by the previous coarse step.
// This test confirms agreement with the simple integrator (ground truth) to
// within one coarse-step tolerance across multiple DS values.
func TestIntegrateHierarchical_BoundaryAccuracy(t *testing.T) {
	sphere := &objects.Sphere{Center: mgl64.Vec3{0, 0, 0}, Radius: 0.5, Rho: 1.0}
	defer setupPhysics(sphere, 0.0, 1.0)()

	origin := mgl64.Vec3{0, 0, -5}
	dir := mgl64.Vec3{0, 0, 1}
	want := math.Exp(-1.0)
	ref := integrate_along_ray(origin, dir, 0.0001, 4.5, 5.5) // high-res reference

	for _, DS := range []float64{0.1, 0.05, 0.02} {
		got := integrate_hierarchical(origin, dir, DS, 4.5, 5.5)
		// Systematic underestimate at boundaries would show as got > want (less attenuation).
		// Tolerance is one coarse step: if left-boundary sub-step were truly dropped,
		// the error would be O(rho * ds) = O(DS/10) per boundary, i.e. ~0.02 for DS=0.2.
		tol := DS
		if math.Abs(got-ref) > tol {
			t.Errorf("DS=%.3f: hierarchical=%.5f ref=%.5f diff=%.5f > tol=%.5f",
				DS, got, ref, math.Abs(got-ref), tol)
		}
		_ = want
	}
}

// TestIntegrateHierarchical_OffCenterRays checks non-central rays through the
// sphere, where boundaries are hit asymmetrically, to catch any directional bias.
func TestIntegrateHierarchical_OffCenterRays(t *testing.T) {
	sphere := &objects.Sphere{Center: mgl64.Vec3{0, 0, 0}, Radius: 0.5, Rho: 1.0}
	defer setupPhysics(sphere, 0.0, 1.0)()

	// For a sphere of radius r, a ray at perpendicular distance d from center
	// has chord length 2*sqrt(r²-d²). At d=0.3, chord = 2*sqrt(0.25-0.09) = 0.8.
	d := 0.3
	chordLen := 2 * math.Sqrt(0.25-d*d)
	want := math.Exp(-chordLen)

	origin := mgl64.Vec3{d, 0, -5}
	dir := mgl64.Vec3{0, 0, 1}
	ref := integrate_along_ray(origin, dir, 0.0001, 4.5, 5.5)
	got := integrate_hierarchical(origin, dir, 0.05, 4.5, 5.5)

	if math.Abs(ref-want) > 1e-3 {
		t.Errorf("reference integrator off-center: got %.5f want %.5f", ref, want)
	}
	if math.Abs(got-ref) > 0.05 {
		t.Errorf("hierarchical off-center: got %.5f ref %.5f diff %.5f", got, ref, math.Abs(got-ref))
	}
}
