//go:build linux

package service

func New() (Manager, error) { return newSystemd() }
