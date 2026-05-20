//go:build darwin

package service

func New() (Manager, error) { return newLaunchd() }
