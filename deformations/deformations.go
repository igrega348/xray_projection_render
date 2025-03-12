package deformations

import (
	"fmt"
	"log"
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
	y0 := y - g.Centers[1]
	z0 := z - g.Centers[2]
	r2 := x0*x0 + y0*y0 + z0*z0
	dx := g.Amplitudes[0] * math.Exp(-r2/(2*g.Sigmas[0]*g.Sigmas[0]))
	dy := g.Amplitudes[1] * math.Exp(-r2/(2*g.Sigmas[1]*g.Sigmas[1]))
	dz := g.Amplitudes[2] * math.Exp(-r2/(2*g.Sigmas[2]*g.Sigmas[2]))
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
	_x := x + l.Strains[0]*x + l.Strains[5]*y + l.Strains[4]*z
	_y := y + l.Strains[5]*x + l.Strains[1]*y + l.Strains[3]*z
	_z := z + l.Strains[4]*x + l.Strains[3]*y + l.Strains[2]*z
	return _x, _y, _z
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

type SigmoidDeformation struct {
	Deformation
	Amplitude   float64
	Center      float64
	Lengthscale float64
	Direction   string
	Type        string
}

func (s *SigmoidDeformation) Apply(x, y, z float64) (float64, float64, float64) {
	switch s.Direction {
	case "x":
		return x + s.Amplitude/(1+math.Exp(-(x-s.Center)/s.Lengthscale)), y, z
	case "y":
		return x, y + s.Amplitude/(1+math.Exp(-(y-s.Center)/s.Lengthscale)), z
	case "z":
		return x, y, z + s.Amplitude/(1+math.Exp(-(z-s.Center)/s.Lengthscale))
	default:
		log.Fatal("Invalid direction")
		return 0, 0, 0
	}
}

func (s *SigmoidDeformation) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"amplitude":   s.Amplitude,
		"center":      s.Center,
		"lengthscale": s.Lengthscale,
		"direction":   s.Direction,
		"type":        s.Type,
	}
}

func (s *SigmoidDeformation) FromMap(data map[string]interface{}) error {
	// check if the data is valid
	var ok bool
	var err error
	if s.Amplitude, err = toFloat64(data["amplitude"]); err != nil {
		return fmt.Errorf("amplitude must be a float")
	}
	if s.Center, err = toFloat64(data["center"]); err != nil {
		return fmt.Errorf("center must be a float")
	}
	if s.Lengthscale, err = toFloat64(data["lengthscale"]); err != nil {
		return fmt.Errorf("lengthscale must be a float")
	}
	if s.Direction, ok = data["direction"].(string); !ok {
		return fmt.Errorf("direction must be a string")
	}
	if s.Type, ok = data["type"].(string); !ok {
		return fmt.Errorf("type must be a string")
	}
	return nil
}

type ComposedDeformation struct {
	Deformation
	Deformations []Deformation
}

func (c *ComposedDeformation) Apply(x, y, z float64) (float64, float64, float64) {
	for _, d := range c.Deformations {
		x, y, z = d.Apply(x, y, z)
	}
	return x, y, z
}

func (c *ComposedDeformation) ToMap() map[string]interface{} {
	deformations := make([]map[string]interface{}, len(c.Deformations))
	for i, d := range c.Deformations {
		deformations[i] = d.ToMap()
	}
	return map[string]interface{}{
		"deformations": deformations,
	}
}

func (c *ComposedDeformation) FromMap(data map[string]interface{}) error {
	deformations, ok := data["deformations"].([]interface{})
	if !ok {
		return fmt.Errorf("deformations must be a list")
	}
	c.Deformations = make([]Deformation, len(deformations))
	for i, d := range deformations {
		deformation, err := NewDeformation(d.(map[string]interface{}))
		if err != nil {
			return err
		}
		c.Deformations[i] = deformation
	}
	return nil
}

type DeformationFactory struct{}

func (f *DeformationFactory) Create(data map[string]interface{}) (Deformation, error) {
	return NewDeformation(data)
}

func NewDeformation(data map[string]interface{}) (Deformation, error) {
	switch data["type"] {
	case "gaussian":
		g := &GaussianDeformation{}
		err := g.FromMap(data)
		return g, err
	case "linear":
		l := &LinearDeformation{}
		err := l.FromMap(data)
		return l, err
	case "rigid":
		r := &RigidDeformation{}
		err := r.FromMap(data)
		return r, err
	case "sigmoid":
		s := &SigmoidDeformation{}
		err := s.FromMap(data)
		return s, err
	case "composed":
		c := &ComposedDeformation{}
		err := c.FromMap(data)
		return c, err
	default:
		return nil, fmt.Errorf("unknown deformation type %s", data["type"])
	}
}

func toFloat64(data interface{}) (float64, error) {
	switch t := data.(type) {
	case int:
		return float64(t), nil
	case float64:
		return t, nil
	default:
		return 0.0, fmt.Errorf("data is not a float64")
	}
}
