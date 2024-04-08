package lattices

import "github.com/go-gl/mathgl/mgl64"

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
