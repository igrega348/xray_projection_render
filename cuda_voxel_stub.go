//go:build !linux || !cgo

package main

import "errors"

func cudaAssembleVolume(res int) ([]float32, error) {
	return nil, errors.New("--use_cuda is only supported on Linux (amd64) with CGO enabled")
}
