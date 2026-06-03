package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ssh-tunnel-service/internal/config"
	"ssh-tunnel-service/internal/paths"
	"ssh-tunnel-service/internal/services"
)

// Provision prepares a freshly installed system service's data root: it creates
// the home tree, ensures a config exists, imports the given private key files
// into the managed key store, and guarantees the system default key exists. It
// runs as part of an elevated install so it writes the system home.
//
// Provision is idempotent — re-running install skips already-imported keys and
// leaves the existing default key in place.
func Provision(home string, importKeyPaths []string) error {
	p, err := paths.Resolve(home)
	if err != nil {
		return err
	}
	if err := p.EnsureTree(); err != nil {
		return fmt.Errorf("prepare home %s: %w", p.Home, err)
	}

	cfg, err := config.LoadWithDefaults(p.Config(), p.KnownHosts())
	if err != nil {
		var miss *config.MissingFileError
		if !errors.As(err, &miss) {
			return err
		}
		if err := config.WriteExample(p.Config(), p.FileMode()); err != nil {
			return fmt.Errorf("init config at %s: %w", p.Config(), err)
		}
		if cfg, err = config.LoadWithDefaults(p.Config(), p.KnownHosts()); err != nil {
			return err
		}
	}

	reg := services.New(cfg, p, services.NewRuntime())
	for _, kp := range importKeyPaths {
		if err := importKeyFile(reg, kp); err != nil {
			return err
		}
	}
	if name, err := reg.EnsureSystemDefaultKey(); err != nil {
		return err
	} else {
		fmt.Fprintf(os.Stderr, "ssh-tunnel: system default key ready (%s)\n", name)
	}
	return nil
}

func importKeyFile(reg *services.Registry, path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read key %s: %w", path, err)
	}
	name := filepath.Base(path)
	err = reg.AddKey(services.SSHKeyInput{
		Name:        name,
		Description: "Imported from " + path,
		PrivateKey:  string(content),
		FileName:    name,
	})
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			fmt.Fprintf(os.Stderr, "ssh-tunnel: key %q already imported, skipping\n", name)
			return nil
		}
		return fmt.Errorf("import key %s: %w", path, err)
	}
	fmt.Fprintf(os.Stderr, "ssh-tunnel: imported key %q\n", name)
	return nil
}
