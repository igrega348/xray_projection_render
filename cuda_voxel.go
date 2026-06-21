//go:build linux && cgo

package main

import (
	"fmt"

	"github.com/igrega348/xray_projection_render/objects"
	"github.com/rs/zerolog/log"
)

// extractCylinders walks an Object hierarchy and returns all Cylinders as a flat list.
func extractCylinders(obj objects.Object) ([]cylinderParams, error) {
	switch v := obj.(type) {
	case *objects.Cylinder:
		return []cylinderParams{{
			p0:     [3]float32{float32(v.P0[0]), float32(v.P0[1]), float32(v.P0[2])},
			p1:     [3]float32{float32(v.P1[0]), float32(v.P1[1]), float32(v.P1[2])},
			radius: float32(v.Radius),
			rho:    float32(v.Rho),
		}}, nil
	case *objects.ObjectCollection:
		var all []cylinderParams
		for _, child := range v.Objects {
			cyls, err := extractCylinders(child)
			if err != nil {
				return nil, err
			}
			all = append(all, cyls...)
		}
		return all, nil
	default:
		return nil, fmt.Errorf("extractCylinders: unsupported object type %T (only ObjectCollection/Cylinder supported for CUDA voxelization)", obj)
	}
}

const spatialHashGridDim = 16 // 16³ = 4096 cells; ~38 cylinders/cell for Kelvin 4x4x4

// cudaAssembleVolume builds a float32 voxel volume on the GPU from lat[0].
// Uses spatial hashing to reduce per-voxel cylinder checks.
// Returned slice has length res³, layout [k*res*res + i*res + j].
func cudaAssembleVolume(res int) ([]float32, error) {
	cyls, err := extractCylinders(lat[0])
	if err != nil {
		return nil, err
	}
	log.Info().Msgf("CUDA voxel assembly (spatial hash G=%d): %d cylinders, res=%d, density_multiplier=%.3f",
		spatialHashGridDim, len(cyls), res, density_multiplier)
	return assembleVoxelGridSpatialCUDA(cyls, res, spatialHashGridDim, density_multiplier)
}
