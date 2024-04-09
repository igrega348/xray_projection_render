package lattices

import (
	"math"

	"github.com/go-gl/mathgl/mgl64"
)

type Strut struct {
	// strut is a line segment with thickness
	P0, P1 mgl64.Vec3
	R      float64
}

type Lattice struct {
	// lattice is a collection of struts
	Struts []Strut
}

func (l *Lattice) Density(x, y, z float64) float64 {
	// for each point, iterate through struts and check if point is
	// within the strut. If so, return 1.0 (density), otherwise 0.0
	for _, strut := range l.Struts {
		// get the vector from the point to the line
		v := strut.P1.Sub(strut.P0)
		w := mgl64.Vec3{x, y, z}.Sub(strut.P0)
		// get the projection of w onto v
		c := w.Dot(v) / v.Dot(v)
		if c < 0.0 || c > 1.0 { // point is definitely not on the line
			continue
		}
		// get the distance from the point to the line
		d := w.Sub(v.Mul(c)).Len()
		if d < strut.R {
			return 1.0
		}
	}
	return 0.0
}

func MakeKelvin(rad float64) Lattice {
	var struts = []Strut{
		{P0: mgl64.Vec3{0.25, 0.00, 0.50}, P1: mgl64.Vec3{0.50, 0.00, 0.75}, R: rad},
		{P0: mgl64.Vec3{0.25, 0.00, 0.50}, P1: mgl64.Vec3{0.50, 0.00, 0.25}, R: rad},
		{P0: mgl64.Vec3{0.25, 0.00, 0.50}, P1: mgl64.Vec3{0.00, 0.25, 0.50}, R: rad},
		{P0: mgl64.Vec3{0.50, 0.00, 0.75}, P1: mgl64.Vec3{0.75, 0.00, 0.50}, R: rad},
		{P0: mgl64.Vec3{0.50, 0.00, 0.75}, P1: mgl64.Vec3{0.50, 0.25, 1.00}, R: rad},
		{P0: mgl64.Vec3{0.75, 0.00, 0.50}, P1: mgl64.Vec3{0.50, 0.00, 0.25}, R: rad},
		{P0: mgl64.Vec3{0.75, 0.00, 0.50}, P1: mgl64.Vec3{1.00, 0.25, 0.50}, R: rad},
		{P0: mgl64.Vec3{0.50, 0.00, 0.25}, P1: mgl64.Vec3{0.50, 0.25, 0.00}, R: rad},
		{P0: mgl64.Vec3{1.00, 0.50, 0.75}, P1: mgl64.Vec3{0.75, 0.50, 1.00}, R: rad},
		{P0: mgl64.Vec3{1.00, 0.75, 0.50}, P1: mgl64.Vec3{0.75, 1.00, 0.50}, R: rad},
		{P0: mgl64.Vec3{1.00, 0.50, 0.25}, P1: mgl64.Vec3{0.75, 0.50, 0.00}, R: rad},
		{P0: mgl64.Vec3{0.25, 1.00, 0.50}, P1: mgl64.Vec3{0.00, 0.75, 0.50}, R: rad},
		{P0: mgl64.Vec3{0.50, 1.00, 0.75}, P1: mgl64.Vec3{0.50, 0.75, 1.00}, R: rad},
		{P0: mgl64.Vec3{0.50, 1.00, 0.25}, P1: mgl64.Vec3{0.50, 0.75, 0.00}, R: rad},
		{P0: mgl64.Vec3{0.00, 0.25, 0.50}, P1: mgl64.Vec3{0.00, 0.50, 0.75}, R: rad},
		{P0: mgl64.Vec3{0.00, 0.25, 0.50}, P1: mgl64.Vec3{0.00, 0.50, 0.25}, R: rad},
		{P0: mgl64.Vec3{0.00, 0.50, 0.75}, P1: mgl64.Vec3{0.25, 0.50, 1.00}, R: rad},
		{P0: mgl64.Vec3{0.00, 0.50, 0.75}, P1: mgl64.Vec3{0.00, 0.75, 0.50}, R: rad},
		{P0: mgl64.Vec3{0.00, 0.75, 0.50}, P1: mgl64.Vec3{0.00, 0.50, 0.25}, R: rad},
		{P0: mgl64.Vec3{0.00, 0.50, 0.25}, P1: mgl64.Vec3{0.25, 0.50, 0.00}, R: rad},
		{P0: mgl64.Vec3{0.25, 0.50, 0.00}, P1: mgl64.Vec3{0.50, 0.75, 0.00}, R: rad},
		{P0: mgl64.Vec3{0.25, 0.50, 0.00}, P1: mgl64.Vec3{0.50, 0.25, 0.00}, R: rad},
		{P0: mgl64.Vec3{0.50, 0.75, 0.00}, P1: mgl64.Vec3{0.75, 0.50, 0.00}, R: rad},
		{P0: mgl64.Vec3{0.75, 0.50, 0.00}, P1: mgl64.Vec3{0.50, 0.25, 0.00}, R: rad},
	}
	return Lattice{Struts: struts}
}

func MakeOctet(rad float64) Lattice {
	s2 := math.Sqrt(2)
	var struts = []Strut{
		{P0: mgl64.Vec3{0, 0, 0}, P1: mgl64.Vec3{0.5, 0.5, -1 / s2}, R: rad},
		{P0: mgl64.Vec3{0, 0, 0}, P1: mgl64.Vec3{1, 0, 0}, R: rad},
		{P0: mgl64.Vec3{0, 0, 0}, P1: mgl64.Vec3{0.5, -0.5, -1 / s2}, R: rad},
		{P0: mgl64.Vec3{0, 0, 0}, P1: mgl64.Vec3{0, 1, 0}, R: rad},
		{P0: mgl64.Vec3{0, 0, 0}, P1: mgl64.Vec3{-0.5, 0.5, -1 / s2}, R: rad},
		{P0: mgl64.Vec3{0, 0, 0}, P1: mgl64.Vec3{0.5, 0.5, 1 / s2}, R: rad},
	}
	return Lattice{Struts: struts}
}
