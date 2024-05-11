package deformations

import (
	"fmt"
	"math"
)

type Deformation interface {
	Apply(x, y, z float64) (float64, float64, float64)
	ToMap() map[string]interface{}
	FromMap(data map[string]interface{}) error
}

type DeformationFactory func() Deformation

var deformations = map[string]DeformationFactory{}

func RegisterDeformation(name string, factory DeformationFactory) {
	deformations[name] = factory
}

func NewDeformation(name string) Deformation {
	if factory, ok := deformations[name]; ok {
		return factory()
	}
	return nil
}

type GaussianDeformation struct {
	Amplitudes []float64
	Sigmas     []float64
	Centers    []float64
}

func (g *GaussianDeformation) Apply(x, y, z float64) (float64, float64, float64) {
	x0 := x - g.Centers[0]
	y0 := y - g.Centers[0]
	z0 := z - g.Centers[0]
	r := math.Sqrt(x0*x0 + y0*y0 + z0*z0)
	dx := g.Amplitudes[0] * math.Exp(-r*r/(2*g.Sigmas[0]*g.Sigmas[0]))
	dy := g.Amplitudes[0] * math.Exp(-r*r/(2*g.Sigmas[0]*g.Sigmas[0]))
	dz := g.Amplitudes[0] * math.Exp(-r*r/(2*g.Sigmas[0]*g.Sigmas[0]))
	return x + dx, y + dy, z + dz
}

func (g *GaussianDeformation) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"amplitudes": g.Amplitudes,
		"sigmas":     g.Sigmas,
		"centers":    g.Centers,
	}
}

func (g *GaussianDeformation) FromMap(data map[string]interface{}) error {
	if amplitudes, ok := data["amplitudes"].([]float64); ok {
		g.Amplitudes = amplitudes
	} else {
		return fmt.Errorf("invalid data for amplitudes")
	}
	if sigmas, ok := data["sigmas"].([]float64); ok {
		g.Sigmas = sigmas
	} else {
		return fmt.Errorf("invalid data for sigmas")
	}
	if centers, ok := data["centers"].([]float64); ok {
		g.Centers = centers
	} else {
		return fmt.Errorf("invalid data for centers")
	}
	return nil
}

func init() {
	RegisterDeformation("gaussian", func() Deformation {
		return &GaussianDeformation{}
	})
}
