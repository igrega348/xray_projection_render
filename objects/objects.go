package objects

import (
	"fmt"
	"math"

	"github.com/go-gl/mathgl/mgl64"
)

type Object interface {
	Density(x, y, z float64) float64
	ToMap() map[string]interface{}
	FromMap(data map[string]interface{}) error
}

type Sphere struct {
	Object
	// parameters are center and radius
	Center mgl64.Vec3
	Radius float64
	Rho    float64
}

func (s *Sphere) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"type":   "sphere",
		"center": s.Center,
		"radius": s.Radius,
		"rho":    s.Rho,
	}
}

func (s *Sphere) FromMap(data map[string]interface{}) error {
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

func (c *Cube) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"type":   "cube",
		"center": c.Center,
		"side":   c.Side,
		"rho":    c.Rho,
	}
}

func (c *Cube) FromMap(data map[string]interface{}) error {
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

func ToVec(data *[]interface{}, vec *mgl64.Vec3) error {
	for i, val := range *data {
		switch t := val.(type) {
		case int:
			vec[i] = float64(t)
		case float64:
			vec[i] = t
		}
	}
	return nil
}

type Cylinder struct {
	Object
	// cylinder is a line segment with thickness
	P0, P1 mgl64.Vec3
	Radius float64
	Rho    float64
}

func (c *Cylinder) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"type":   "cylinder",
		"p0":     c.P0,
		"p1":     c.P1,
		"radius": c.Radius,
		"rho":    c.Rho,
	}
}

func (c *Cylinder) FromMap(data map[string]interface{}) error {
	var ok bool
	var slice []interface{}
	if slice, ok = data["p0"].([]interface{}); !ok {
		return fmt.Errorf("p0 is not a Vec3")
	}
	err := ToVec(&slice, &c.P0)
	if err != nil {
		return err
	}
	if slice, ok = data["p1"].([]interface{}); !ok {
		return fmt.Errorf("p0 is not a Vec3")
	}
	err = ToVec(&slice, &c.P1)
	if err != nil {
		return err
	}
	if c.Radius, ok = data["radius"].(float64); !ok {
		return fmt.Errorf("radius is not a float64")
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
	if d < cyl.Radius {
		return cyl.Rho
	} else {
		return 0.0
	}
}

type ObjectCollection struct {
	Objects []Object
}

func (oc *ObjectCollection) ToMap() map[string]interface{} {
	var objects = make([]map[string]interface{}, len(oc.Objects))
	for i, object := range oc.Objects {
		objects[i] = object.ToMap()
	}
	return map[string]interface{}{
		"type":    "object_collection",
		"objects": objects,
	}
}

func (oc *ObjectCollection) FromMap(data map[string]interface{}) error {
	var objects []Object
	if objects_data, ok := data["objects"].([]interface{}); ok {
		objects = make([]Object, len(objects_data))
		for i, object_data := range objects_data {
			switch object_data.(map[string]interface{})["type"] {
			case "sphere":
				object := Sphere{}
				if err := object.FromMap(object_data.(map[string]interface{})); err != nil {
					return err
				}
				objects[i] = &object
			case "cube":
				object := Cube{}
				if err := object.FromMap(object_data.(map[string]interface{})); err != nil {
					return err
				}
				objects[i] = &object
			case "cylinder":
				object := Cylinder{}
				if err := object.FromMap(object_data.(map[string]interface{})); err != nil {
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

type UnitCell struct {
	// object collection. But overload density method and provide bounds
	Struts                             ObjectCollection
	Xmin, Xmax, Ymin, Ymax, Zmin, Zmax float64
}

func (uc *UnitCell) Density(x, y, z float64) float64 {
	// check if point is within bounds. But account for struts a bit smaller
	if x < uc.Xmin || x > uc.Xmax || y < uc.Ymin || y > uc.Ymax || z < uc.Zmin || z > uc.Zmax {
		return 0.0
	}
	for _, strut := range uc.Struts.Objects {
		rho := strut.Density(x, y, z)
		if rho > 0.0 {
			return rho
		} else {
			continue
		}
	}
	return 0.0
}

func (uc *UnitCell) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"type":   "unit_cell",
		"struts": uc.Struts.ToMap(),
		"xmin":   uc.Xmin,
		"xmax":   uc.Xmax,
		"ymin":   uc.Ymin,
		"ymax":   uc.Ymax,
		"zmin":   uc.Zmin,
		"zmax":   uc.Zmax,
	}
}

func (uc *UnitCell) FromMap(data map[string]interface{}) error {
	var ok bool
	if struts_data, ok := data["struts"].(map[string]interface{}); ok {
		struts := ObjectCollection{}
		if err := struts.FromMap(struts_data); err != nil {
			return err
		}
		uc.Struts = struts
	} else {
		return fmt.Errorf("struts is not a map")
	}
	if uc.Xmin, ok = data["xmin"].(float64); !ok {
		return fmt.Errorf("xmin is not a float64")
	}
	if uc.Xmax, ok = data["xmax"].(float64); !ok {
		return fmt.Errorf("xmax is not a float64")
	}
	if uc.Ymin, ok = data["ymin"].(float64); !ok {
		return fmt.Errorf("ymin is not a float64")
	}
	if uc.Ymax, ok = data["ymax"].(float64); !ok {
		return fmt.Errorf("ymax is not a float64")
	}
	if uc.Zmin, ok = data["zmin"].(float64); !ok {
		return fmt.Errorf("zmin is not a float64")
	}
	if uc.Zmax, ok = data["zmax"].(float64); !ok {
		return fmt.Errorf("zmax is not a float64")
	}
	return nil
}

type Lattice struct {
	// lattice is given by unit cell and bounds for tessellation
	UC                                 UnitCell
	Xmin, Xmax, Ymin, Ymax, Zmin, Zmax float64
}

func (l *Lattice) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"type": "lattice",
		"uc":   l.UC.ToMap(),
		"xmin": l.Xmin,
		"xmax": l.Xmax,
		"ymin": l.Ymin,
		"ymax": l.Ymax,
		"zmin": l.Zmin,
		"zmax": l.Zmax,
	}
}

func (l *Lattice) FromMap(data map[string]interface{}) error {
	var ok bool
	if uc_data, ok := data["uc"].(map[string]interface{}); ok {
		uc := UnitCell{}
		if err := uc.FromMap(uc_data); err != nil {
			return err
		}
		l.UC = uc
	} else {
		return fmt.Errorf("uc is not a map")
	}
	if l.Xmin, ok = data["xmin"].(float64); !ok {
		return fmt.Errorf("xmin is not a float64")
	}
	if l.Xmax, ok = data["xmax"].(float64); !ok {
		return fmt.Errorf("xmax is not a float64")
	}
	if l.Ymin, ok = data["ymin"].(float64); !ok {
		return fmt.Errorf("ymin is not a float64")
	}
	if l.Ymax, ok = data["ymax"].(float64); !ok {
		return fmt.Errorf("ymax is not a float64")
	}
	if l.Zmin, ok = data["zmin"].(float64); !ok {
		return fmt.Errorf("zmin is not a float64")
	}
	if l.Zmax, ok = data["zmax"].(float64); !ok {
		return fmt.Errorf("zmax is not a float64")
	}
	return nil
}

func (l *Lattice) Density(x, y, z float64) float64 {
	// check if point is within bounds
	if x < l.Xmin || x > l.Xmax || y < l.Ymin || y > l.Ymax || z < l.Zmin || z > l.Zmax {
		return 0.0
	} else {
		// map point to unit cell
		dx := l.UC.Xmax - l.UC.Xmin
		x = x - dx*math.Floor((x-l.UC.Xmin)/dx)
		dy := l.UC.Ymax - l.UC.Ymin
		y = y - dy*math.Floor((y-l.UC.Ymin)/dy)
		dz := l.UC.Zmax - l.UC.Zmin
		z = z - dz*math.Floor((z-l.UC.Zmin)/dz)
		return l.UC.Density(x, y, z)
	}
}

func MakeKelvin(rad float64, scale float64) UnitCell {
	var struts = []Cylinder{
		{P0: mgl64.Vec3{0.25, 0.00, 0.50}, P1: mgl64.Vec3{0.50, 0.00, 0.75}, Radius: rad, Rho: 1.0},
		{P0: mgl64.Vec3{0.25, 1.00, 0.50}, P1: mgl64.Vec3{0.50, 1.00, 0.75}, Radius: rad, Rho: 1.0},
		{P0: mgl64.Vec3{0.25, 0.00, 0.50}, P1: mgl64.Vec3{0.50, 0.00, 0.25}, Radius: rad, Rho: 1.0},
		{P0: mgl64.Vec3{0.25, 1.00, 0.50}, P1: mgl64.Vec3{0.50, 1.00, 0.25}, Radius: rad, Rho: 1.0},
		{P0: mgl64.Vec3{0.25, 0.00, 0.50}, P1: mgl64.Vec3{0.00, 0.25, 0.50}, Radius: rad, Rho: 1.0},
		{P0: mgl64.Vec3{0.50, 0.00, 0.75}, P1: mgl64.Vec3{0.75, 0.00, 0.50}, Radius: rad, Rho: 1.0},
		{P0: mgl64.Vec3{0.50, 1.00, 0.75}, P1: mgl64.Vec3{0.75, 1.00, 0.50}, Radius: rad, Rho: 1.0},
		{P0: mgl64.Vec3{0.50, 0.00, 0.75}, P1: mgl64.Vec3{0.50, 0.25, 1.00}, Radius: rad, Rho: 1.0},
		{P0: mgl64.Vec3{0.75, 0.00, 0.50}, P1: mgl64.Vec3{0.50, 0.00, 0.25}, Radius: rad, Rho: 1.0},
		{P0: mgl64.Vec3{0.75, 1.00, 0.50}, P1: mgl64.Vec3{0.50, 1.00, 0.25}, Radius: rad, Rho: 1.0},
		{P0: mgl64.Vec3{0.75, 0.00, 0.50}, P1: mgl64.Vec3{1.00, 0.25, 0.50}, Radius: rad, Rho: 1.0},
		{P0: mgl64.Vec3{0.50, 0.00, 0.25}, P1: mgl64.Vec3{0.50, 0.25, 0.00}, Radius: rad, Rho: 1.0},
		{P0: mgl64.Vec3{1.00, 0.50, 0.75}, P1: mgl64.Vec3{0.75, 0.50, 1.00}, Radius: rad, Rho: 1.0},
		{P0: mgl64.Vec3{1.00, 0.75, 0.50}, P1: mgl64.Vec3{0.75, 1.00, 0.50}, Radius: rad, Rho: 1.0},
		{P0: mgl64.Vec3{1.00, 0.50, 0.25}, P1: mgl64.Vec3{0.75, 0.50, 0.00}, Radius: rad, Rho: 1.0},
		{P0: mgl64.Vec3{0.25, 1.00, 0.50}, P1: mgl64.Vec3{0.00, 0.75, 0.50}, Radius: rad, Rho: 1.0},
		{P0: mgl64.Vec3{0.50, 1.00, 0.75}, P1: mgl64.Vec3{0.50, 0.75, 1.00}, Radius: rad, Rho: 1.0},
		{P0: mgl64.Vec3{0.50, 1.00, 0.25}, P1: mgl64.Vec3{0.50, 0.75, 0.00}, Radius: rad, Rho: 1.0},
		{P0: mgl64.Vec3{0.00, 0.25, 0.50}, P1: mgl64.Vec3{0.00, 0.50, 0.75}, Radius: rad, Rho: 1.0},
		{P0: mgl64.Vec3{1.00, 0.25, 0.50}, P1: mgl64.Vec3{1.00, 0.50, 0.75}, Radius: rad, Rho: 1.0},
		{P0: mgl64.Vec3{0.00, 0.25, 0.50}, P1: mgl64.Vec3{0.00, 0.50, 0.25}, Radius: rad, Rho: 1.0},
		{P0: mgl64.Vec3{1.00, 0.25, 0.50}, P1: mgl64.Vec3{1.00, 0.50, 0.25}, Radius: rad, Rho: 1.0},
		{P0: mgl64.Vec3{0.00, 0.50, 0.75}, P1: mgl64.Vec3{0.25, 0.50, 1.00}, Radius: rad, Rho: 1.0},
		{P0: mgl64.Vec3{0.00, 0.50, 0.75}, P1: mgl64.Vec3{0.00, 0.75, 0.50}, Radius: rad, Rho: 1.0},
		{P0: mgl64.Vec3{1.00, 0.50, 0.75}, P1: mgl64.Vec3{1.00, 0.75, 0.50}, Radius: rad, Rho: 1.0},
		{P0: mgl64.Vec3{0.00, 0.75, 0.50}, P1: mgl64.Vec3{0.00, 0.50, 0.25}, Radius: rad, Rho: 1.0},
		{P0: mgl64.Vec3{1.00, 0.75, 0.50}, P1: mgl64.Vec3{1.00, 0.50, 0.25}, Radius: rad, Rho: 1.0},
		{P0: mgl64.Vec3{0.00, 0.50, 0.25}, P1: mgl64.Vec3{0.25, 0.50, 0.00}, Radius: rad, Rho: 1.0},
		{P0: mgl64.Vec3{0.25, 0.50, 0.00}, P1: mgl64.Vec3{0.50, 0.75, 0.00}, Radius: rad, Rho: 1.0},
		{P0: mgl64.Vec3{0.25, 0.50, 1.00}, P1: mgl64.Vec3{0.50, 0.75, 1.00}, Radius: rad, Rho: 1.0},
		{P0: mgl64.Vec3{0.25, 0.50, 0.00}, P1: mgl64.Vec3{0.50, 0.25, 0.00}, Radius: rad, Rho: 1.0},
		{P0: mgl64.Vec3{0.25, 0.50, 1.00}, P1: mgl64.Vec3{0.50, 0.25, 1.00}, Radius: rad, Rho: 1.0},
		{P0: mgl64.Vec3{0.50, 0.75, 0.00}, P1: mgl64.Vec3{0.75, 0.50, 0.00}, Radius: rad, Rho: 1.0},
		{P0: mgl64.Vec3{0.50, 0.75, 1.00}, P1: mgl64.Vec3{0.75, 0.50, 1.00}, Radius: rad, Rho: 1.0},
		{P0: mgl64.Vec3{0.75, 0.50, 0.00}, P1: mgl64.Vec3{0.50, 0.25, 0.00}, Radius: rad, Rho: 1.0},
		{P0: mgl64.Vec3{0.75, 0.50, 1.00}, P1: mgl64.Vec3{0.50, 0.25, 1.00}, Radius: rad, Rho: 1.0},
	}
	for i := 0; i < len(struts); i++ {
		struts[i].P0 = struts[i].P0.Mul(scale)
		struts[i].P1 = struts[i].P1.Mul(scale)
	}
	var objects = make([]Object, len(struts))
	for i, strut := range struts {
		objects[i] = &strut
	}
	uc := UnitCell{Struts: ObjectCollection{Objects: objects}, Xmin: 0.0, Xmax: 1.0 * scale, Ymin: 0.0, Ymax: 1.0 * scale, Zmin: 0.0, Zmax: 1.0 * scale}
	return uc
}

// func MakeOctet(rad float64) Lattice {
// 	s2 := math.Sqrt(2)
// 	var struts = []Cylinder{
// 		{P0: mgl64.Vec3{0, 0, 0}, P1: mgl64.Vec3{0.5, 0.5, -1 / s2}, Radius: rad},
// 		{P0: mgl64.Vec3{0, 0, 0}, P1: mgl64.Vec3{1, 0, 0}, Radius: rad},
// 		{P0: mgl64.Vec3{0, 0, 0}, P1: mgl64.Vec3{0.5, -0.5, -1 / s2}, Radius: rad},
// 		{P0: mgl64.Vec3{0, 0, 0}, P1: mgl64.Vec3{0, 1, 0}, Radius: rad},
// 		{P0: mgl64.Vec3{0, 0, 0}, P1: mgl64.Vec3{-0.5, 0.5, -1 / s2}, Radius: rad},
// 		{P0: mgl64.Vec3{0, 0, 0}, P1: mgl64.Vec3{0.5, 0.5, 1 / s2}, Radius: rad},
// 	}
// 	return Lattice{Struts: struts}
// }
