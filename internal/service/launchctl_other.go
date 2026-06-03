//go:build !darwin

package service

func darwinBootout()             {}
func darwinBootstrap() error     { return nil }
func darwinStart() (bool, error) { return false, nil }
func darwinStop() (bool, error)  { return false, nil }
