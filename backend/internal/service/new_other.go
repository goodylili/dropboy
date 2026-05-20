//go:build !darwin && !linux

package service

import "errors"

func New() (Manager, error) {
	return nil, errors.New("service install not supported on this platform")
}
