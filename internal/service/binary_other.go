//go:build !linux

package service

func installSystemBinary() (string, error) { return "", nil }
func removeSystemBinary() error             { return nil }
