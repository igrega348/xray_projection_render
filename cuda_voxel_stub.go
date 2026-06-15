//go:build !cuda
// +build !cuda

package main

import "errors"

func cudaAssembleVolume(res int) ([]float32, error) {
	return nil, errors.New("not compiled with CUDA support; rebuild with -tags=cuda")
}
