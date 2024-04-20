package objects

import (
	"fmt"
	"math"

	"github.com/go-gl/mathgl/mgl64"
)

type Object interface {
	Density(x, y, z float64) float64
	ToYAML() map[string]interface{}
	FromYAML(data map[string]interface{}) error
}

type Sphere struct {
	Object
	// parameters are center and radius
	Center mgl64.Vec3
	Radius float64
	Rho    float64
}

func (s *Sphere) ToYAML() map[string]interface{} {
	return map[string]interface{}{
		"type":   "sphere",
		"center": s.Center,
		"radius": s.Radius,
		"rho":    s.Rho,
	}
}

func (s *Sphere) FromYAML(data map[string]interface{}) error {
	var ok bool
	var slice []interface{}
	if slice, ok = data["center"].([]interface{}); !ok {
		return fmt.Errorf("center is not a Vec3")
	}
	for i, val := range slice {
		s.Center[i] = val.(float64)
	}
	if s.Radius, ok = data["radius"].(float64); !ok {
		return fmt.Errorf("radius is not a float64")
	}
	if s.Rho, ok = data["rho"].(float64); !ok {
		return fmt.Errorf("rho is not a float64")
	}
	return nil
}

func (s *Sphere) Density(x, y, z float64) float64 {
	x = x - s.Center[0]
	y = y - s.Center[1]
	z = z - s.Center[2]
	r_2 := x*x + y*y + z*z
	if r_2 < s.Radius*s.Radius {
		return s.Rho
	}
	return 0.0
}

type Cube struct {
	Object
	// parameters are center and side length
	Center mgl64.Vec3
	Side   float64
	Rho    float64
}

func (c *Cube) ToYAML() map[string]interface{} {
	return map[string]interface{}{
		"type":   "cube",
		"center": c.Center,
		"side":   c.Side,
		"rho":    c.Rho,
	}
}

func (c *Cube) FromYAML(data map[string]interface{}) error {
	var ok bool
	var slice []interface{}
	if slice, ok = data["center"].([]interface{}); !ok {
		return fmt.Errorf("center is not a Vec3")
	}
	for i, val := range slice {
		c.Center[i] = val.(float64)
	}
	if c.Side, ok = data["side"].(float64); !ok {
		return fmt.Errorf("side is not a float64")
	}
	if c.Rho, ok = data["rho"].(float64); !ok {
		return fmt.Errorf("rho is not a float64")
	}
	return nil
}

func (c *Cube) Density(x, y, z float64) float64 {
	x = math.Abs(x - c.Center[0])
	y = math.Abs(y - c.Center[1])
	z = math.Abs(z - c.Center[2])
	if x < 0.5*c.Side && y < 0.5*c.Side && z < 0.5*c.Side {
		return c.Rho
	}
	return 0.0
}

type Cylinder struct {
	Object
	// cylinder is a line segment with thickness
	P0, P1 mgl64.Vec3
	R      float64
	Rho    float64
}

func (c *Cylinder) ToYAML() map[string]interface{} {
	return map[string]interface{}{
		"type": "cylinder",
		"p0":   c.P0,
		"p1":   c.P1,
		"r":    c.R,
		"rho":  c.Rho,
	}
}

func (c *Cylinder) FromYAML(data map[string]interface{}) error {
	var ok bool
	var slice []interface{}
	if slice, ok = data["p0"].([]interface{}); !ok {
		return fmt.Errorf("p0 is not a Vec3")
	}
	for i, val := range slice {
		c.P0[i] = val.(float64)
	}
	if slice, ok = data["p1"].([]interface{}); !ok {
		return fmt.Errorf("p1 is not a Vec3")
	}
	for i, val := range slice {
		c.P1[i] = val.(float64)
	}
	if c.R, ok = data["r"].(float64); !ok {
		return fmt.Errorf("r is not a float64")
	}
	if c.Rho, ok = data["rho"].(float64); !ok {
		return fmt.Errorf("rho is not a float64")
	}
	return nil
}

func (cyl *Cylinder) Density(x, y, z float64) float64 {
	// get the vector from the point to the line
	v := cyl.P1.Sub(cyl.P0)
	w := mgl64.Vec3{x, y, z}.Sub(cyl.P0)
	// get the projection of w onto v
	c := w.Dot(v) / v.Dot(v)
	if c < 0.0 || c > 1.0 { // point is definitely not on the line
		return 0.0
	}
	// get the distance from the point to the line
	d := w.Sub(v.Mul(c)).Len()
	if d < cyl.R {
		return cyl.Rho
	} else {
		return 0.0
	}
}

type ObjectCollection struct {
	Objects []Object
}

func (oc *ObjectCollection) ToYAML() map[string]interface{} {
	var objects = make([]map[string]interface{}, len(oc.Objects))
	for i, object := range oc.Objects {
		objects[i] = object.ToYAML()
	}
	return map[string]interface{}{
		"type":    "object_collection",
		"objects": objects,
	}
}

func (oc *ObjectCollection) FromYAML(data map[string]interface{}) error {
	var objects []Object
	if objects_data, ok := data["objects"].([]interface{}); ok {
		objects = make([]Object, len(objects_data))
		for i, object_data := range objects_data {
			switch object_data.(map[string]interface{})["type"] {
			case "sphere":
				object := Sphere{}
				if err := object.FromYAML(object_data.(map[string]interface{})); err != nil {
					return err
				}
				objects[i] = &object
			case "cube":
				object := Cube{}
				if err := object.FromYAML(object_data.(map[string]interface{})); err != nil {
					return err
				}
				objects[i] = &object
			case "cylinder":
				object := Cylinder{}
				if err := object.FromYAML(object_data.(map[string]interface{})); err != nil {
					return err
				}
				objects[i] = &object
			default:
				return fmt.Errorf("unknown object type")
			}
		}
	} else {
		return fmt.Errorf("objects is not a list")
	}
	oc.Objects = objects
	return nil
}

func (oc *ObjectCollection) Density(x, y, z float64) float64 {
	var density float64
	for _, object := range oc.Objects {
		density += object.Density(x, y, z)
	}
	// clip between 0 and 1
	if density < 0.0 {
		density = 0.0
	} else if density > 1.0 {
		density = 1.0
	}
	return density
}

type Lattice struct {
	// lattice is a collection of struts
	// It's good to have a separate class for lattice because
	// if we don't allow negative volumes, we can have faster iteration
	Struts []Cylinder
}

func (l *Lattice) ToYAML() map[string]interface{} {
	var struts = make([]map[string]interface{}, len(l.Struts))
	for i, strut := range l.Struts {
		struts[i] = strut.ToYAML()
	}
	return map[string]interface{}{
		"type":   "lattice",
		"struts": struts,
	}
}

func (l *Lattice) FromYAML(data map[string]interface{}) error {
	var struts []Cylinder
	if struts_data, ok := data["struts"].([]interface{}); ok {
		struts = make([]Cylinder, len(struts_data))
		for i, strut_data := range struts_data {
			strut := Cylinder{}
			if err := strut.FromYAML(strut_data.(map[string]interface{})); err != nil {
				return err
			}
			struts[i] = strut
		}
	} else {
		return fmt.Errorf("struts is not a list")
	}
	l.Struts = struts
	return nil
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

func (lat *Lattice) Tesselate(nx, ny, nz int) Lattice {
	scaler := 1.0 / float64(max(nx, ny, nz))
	dx := mgl64.Vec3{1, 0, 0}
	dy := mgl64.Vec3{0, 1, 0}
	dz := mgl64.Vec3{0, 0, 1}
	var tess = make([]Cylinder, nx*ny*nz*len(lat.Struts))
	for i := 0; i < nx; i++ {
		for j := 0; j < ny; j++ {
			for k := 0; k < nz; k++ {
				for i_s := 0; i_s < len(lat.Struts); i_s++ {
					dr := dx.Mul(float64(i)).Add(dy.Mul(float64(j)).Add(dz.Mul(float64(k))))
					tess[(i*ny*nz+j*nz+k)*len(lat.Struts)+i_s] = Cylinder{
						P0: lat.Struts[i_s].P0.Add(dr).Mul(scaler).Sub(mgl64.Vec3{0.5, 0.5, 0.5}),
						P1: lat.Struts[i_s].P1.Add(dr).Mul(scaler).Sub(mgl64.Vec3{0.5, 0.5, 0.5}),
						R:  lat.Struts[i_s].R * scaler}
				}
			}
		}
	}
	return Lattice{Struts: tess}
}

func MakeKelvin(rad float64) Lattice {
	var struts = []Cylinder{
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
	var struts = []Cylinder{
		{P0: mgl64.Vec3{0, 0, 0}, P1: mgl64.Vec3{0.5, 0.5, -1 / s2}, R: rad},
		{P0: mgl64.Vec3{0, 0, 0}, P1: mgl64.Vec3{1, 0, 0}, R: rad},
		{P0: mgl64.Vec3{0, 0, 0}, P1: mgl64.Vec3{0.5, -0.5, -1 / s2}, R: rad},
		{P0: mgl64.Vec3{0, 0, 0}, P1: mgl64.Vec3{0, 1, 0}, R: rad},
		{P0: mgl64.Vec3{0, 0, 0}, P1: mgl64.Vec3{-0.5, 0.5, -1 / s2}, R: rad},
		{P0: mgl64.Vec3{0, 0, 0}, P1: mgl64.Vec3{0.5, 0.5, 1 / s2}, R: rad},
	}
	return Lattice{Struts: struts}
}
