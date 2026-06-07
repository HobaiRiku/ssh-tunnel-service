//go:build !darwin

package service

func darwinBootout(bool)             {}
func darwinBootstrap(bool) error     { return nil }
func darwinStart(bool) (bool, error) { return false, nil }
func darwinStop(bool) (bool, error)  { return false, nil }
