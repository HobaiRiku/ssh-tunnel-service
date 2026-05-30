//go:build !darwin

package service

func darwinBootout()                 {}
func darwinBootstrap() error         { return nil }
func darwinKickstart() (bool, error) { return false, nil }
func darwinKill() (bool, error)      { return false, nil }
