package deformations

import (
	"math"
	"testing"
)

// TestAffineDeformationIsLinearMap verifies that AffineDeformation.Apply is a
// pure linear coordinate transform (M*v), NOT a displacement field. An identity
// matrix must return the input unchanged; a 2x x-scale must double x only.
func TestAffineDeformationIsLinearMap(t *testing.T) {
	identity := &AffineDeformation{
		Matrix: [3][3]float64{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}},
	}
	x, y, z := identity.Apply(3, 4, 5)
	if x != 3 || y != 4 || z != 5 {
		t.Errorf("identity: got (%.3f,%.3f,%.3f) want (3,4,5)", x, y, z)
	}

	// Scale x by 2 — this is a COORDINATE transform, not a displacement.
	// A user wanting a 0.1 x-displacement would need M = [[1.1,0,0],[0,1,0],[0,0,1]],
	// which produces 1.1*x, NOT x+0.1.
	scaleX := &AffineDeformation{
		Matrix: [3][3]float64{{2, 0, 0}, {0, 1, 0}, {0, 0, 1}},
	}
	x, y, z = scaleX.Apply(3, 4, 5)
	if x != 6 || y != 4 || z != 5 {
		t.Errorf("2x x-scale: got (%.3f,%.3f,%.3f) want (6,4,5)", x, y, z)
	}
}

// TestGaussianDeformationIsDisplacementField verifies that GaussianDeformation
// adds a displacement to the input (contrast with AffineDeformation above).
func TestGaussianDeformationIsDisplacementField(t *testing.T) {
	g := &GaussianDeformation{
		Amplitudes: []float64{0.5, 0, 0},
		Sigmas:     []float64{1.0, 1.0, 1.0},
		Centers:    []float64{0, 0, 0},
	}
	x, y, z := g.Apply(0, 0, 0)
	// At center: displacement = amplitude * exp(0) = 0.5 added to x.
	if math.Abs(x-0.5) > 1e-9 || y != 0 || z != 0 {
		t.Errorf("Gaussian at center: got (%.4f,%.4f,%.4f) want (0.5,0,0)", x, y, z)
	}

	// Zero amplitudes → identity (no displacement).
	g0 := &GaussianDeformation{
		Amplitudes: []float64{0, 0, 0},
		Sigmas:     []float64{1.0, 1.0, 1.0},
		Centers:    []float64{0, 0, 0},
	}
	x, y, z = g0.Apply(3, 4, 5)
	if x != 3 || y != 4 || z != 5 {
		t.Errorf("zero-amplitude Gaussian: got (%.3f,%.3f,%.3f) want (3,4,5)", x, y, z)
	}
}
