package config

import (
	"errors"
	"fmt"
	"os"
)

// MissingFileError is returned when config.yaml doesn't exist yet.
type MissingFileError struct{ Path string }

func (e *MissingFileError) Error() string {
	return fmt.Sprintf("config file not found: %s", e.Path)
}

// Load reads and validates config from path.
func Load(path string) (*Config, error) {
	return LoadWithDefaults(path, "")
}

// LoadWithDefaults reads config from path, applies defaults, then validates it.
func LoadWithDefaults(path, defaultKnownHosts string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, &MissingFileError{Path: path}
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	cfg, err := parseConfig(data)
	if err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	ApplyDefaults(cfg, defaultKnownHosts)
	if err := Validate(cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return cfg, nil
}
