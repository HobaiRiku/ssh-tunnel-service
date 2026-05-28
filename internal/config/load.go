package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// MissingFileError is returned when config.yaml doesn't exist yet.
type MissingFileError struct{ Path string }

func (e *MissingFileError) Error() string {
	return fmt.Sprintf("config file not found: %s", e.Path)
}

// Load reads and validates config from path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, &MissingFileError{Path: path}
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	ApplyDefaults(&cfg, "")
	if err := Validate(&cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return &cfg, nil
}
