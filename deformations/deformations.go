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

type GaussianDeformation struct {
	Deformation
	Amplitudes []float64
	Sigmas     []float64
	Centers    []float64
	Type       string
}

func (g *GaussianDeformation) Apply(x, y, z float64) (float64, float64, float64) {
	x0 := x - g.Centers[0]
	y0 := y - g.Centers[0]
	z0 := z - g.Centers[0]
	r := math.Sqrt(x0*x0 + y0*y0 + z0*z0)
	dx := g.Amplitudes[0] * math.Exp(-r*r/(2*g.Sigmas[0]*g.Sigmas[0]))
	dy := g.Amplitudes[1] * math.Exp(-r*r/(2*g.Sigmas[1]*g.Sigmas[1]))
	dz := g.Amplitudes[2] * math.Exp(-r*r/(2*g.Sigmas[2]*g.Sigmas[2]))
	return x + dx, y + dy, z + dz
}

func (g *GaussianDeformation) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"amplitudes": g.Amplitudes,
		"sigmas":     g.Sigmas,
		"centers":    g.Centers,
		"type":       g.Type,
	}
}

func (g *GaussianDeformation) FromMap(data map[string]interface{}) error {
	amplitudes, ok := data["amplitudes"].([]interface{})
	if !ok {
		return fmt.Errorf("amplitudes must be a list")
	}
	g.Amplitudes = make([]float64, len(amplitudes))
	for i, a := range amplitudes {
		g.Amplitudes[i] = a.(float64)
	}
	sigmas := data["sigmas"].([]interface{})
	if !ok {
		return fmt.Errorf("sigmas must be a list")
	}
	g.Sigmas = make([]float64, len(sigmas))
	for i, s := range sigmas {
		g.Sigmas[i] = s.(float64)
	}
	centers := data["centers"].([]interface{})
	if !ok {
		return fmt.Errorf("centers must be a list")
	}
	g.Centers = make([]float64, len(centers))
	for i, c := range centers {
		g.Centers[i] = c.(float64)
	}
	g.Type = data["type"].(string)
	return nil
}

type LinearDeformation struct {
	Deformation
	Strains []float64
	Type    string
}

func (l *LinearDeformation) Apply(x, y, z float64) (float64, float64, float64) {
	return x + l.Strains[0]*x, y + l.Strains[1]*y, z + l.Strains[2]*z
}

func (l *LinearDeformation) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"strains": l.Strains,
		"type":    l.Type,
	}
}

func (l *LinearDeformation) FromMap(data map[string]interface{}) error {
	strains, ok := data["strains"].([]interface{})
	if !ok {
		return fmt.Errorf("strains must be a list")
	}
	l.Strains = make([]float64, len(strains))
	for i, s := range strains {
		l.Strains[i] = s.(float64)
	}
	l.Type = data["type"].(string)
	return nil
}

type RigidDeformation struct {
	Deformation
	Displacements []float64
	Type          string
}

func (r *RigidDeformation) Apply(x, y, z float64) (float64, float64, float64) {
	return x + r.Displacements[0], y + r.Displacements[1], z + r.Displacements[2]
}

func (r *RigidDeformation) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"displacements": r.Displacements,
		"type":          r.Type,
	}
}

func (r *RigidDeformation) FromMap(data map[string]interface{}) error {
	displacements, ok := data["displacements"].([]interface{})
	if !ok {
		return fmt.Errorf("displacements must be a list")
	}
	r.Displacements = make([]float64, len(displacements))
	for i, d := range displacements {
		r.Displacements[i] = d.(float64)
	}
	r.Type = data["type"].(string)
	return nil
}
