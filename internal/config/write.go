package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// WriteRaw atomically writes raw YAML bytes to path with the given mode.
func WriteRaw(path string, data []byte, mode os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return fmt.Errorf("write tmp %s: %w", tmp, err)
	}
	// os.WriteFile honors the process umask, which can strip the group-write bit
	// that makes config.yaml admin-editable on the macOS system service. Chmod to
	// the exact mode so the on-disk permissions match the caller's intent.
	if err := os.Chmod(tmp, mode); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("chmod tmp %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename %s -> %s: %w", tmp, path, err)
	}
	return nil
}

// Write marshals cfg to path atomically.
func Write(path string, cfg *Config, mode os.FileMode) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return WriteRaw(path, data, mode)
}
