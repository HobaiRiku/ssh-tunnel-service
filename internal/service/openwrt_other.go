//go:build !linux

package service

import "fmt"

// isOpenWrt always returns false on non-Linux platforms.
func isOpenWrt() bool { return false }

func openwrtInstall(string) error {
	return fmt.Errorf("OpenWrt procd services are only supported on Linux")
}

func openwrtUninstall() error {
	return fmt.Errorf("OpenWrt procd services are only supported on Linux")
}

func openwrtStart() error {
	return fmt.Errorf("OpenWrt procd services are only supported on Linux")
}

func openwrtStop() error {
	return fmt.Errorf("OpenWrt procd services are only supported on Linux")
}

func openwrtRunService(string) error {
	return fmt.Errorf("OpenWrt procd services are only supported on Linux")
}
