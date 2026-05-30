package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"ssh-tunnel-service/internal/config"
	"ssh-tunnel-service/internal/paths"
)

func configCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "config",
		Short: "Manage service configuration",
	}
	root.AddCommand(configShowCmd(), configEditCmd(), configPathCmd(), configKnownHostsPathCmd())
	return root
}

func configShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Print current configuration as YAML",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, _, err := loadConfigForCLI(rootFlags.Home)
			if err != nil {
				return err
			}
			return yaml.NewEncoder(os.Stdout).Encode(cfg)
		},
	}
}

func configEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Open config in $EDITOR",
		RunE: func(_ *cobra.Command, _ []string) error {
			_, p, err := loadConfigForCLI(rootFlags.Home)
			if err != nil {
				return err
			}
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}
			cmd := exec.Command(editor, p.Config())
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		},
	}
}

func configKnownHostsPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "known-hosts-path",
		Short: "Print the effective managed known_hosts file path",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, _, err := loadConfigForCLI(rootFlags.Home)
			if err != nil {
				return err
			}
			fmt.Println(cfg.App.SSHKnownHosts)
			return nil
		},
	}
}

func configPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print config file path",
		RunE: func(_ *cobra.Command, _ []string) error {
			p, err := paths.Resolve(rootFlags.Home)
			if err != nil {
				return err
			}
			fmt.Println(p.Config())
			return nil
		},
	}
}

func loadConfigForCLI(home string) (*config.Config, paths.Paths, error) {
	p, err := paths.Resolve(home)
	if err != nil {
		return nil, paths.Paths{}, err
	}
	if err := p.EnsureTree(); err != nil {
		return nil, paths.Paths{}, err
	}
	cfg, err := config.LoadWithDefaults(p.Config(), p.KnownHosts())
	if err != nil {
		var miss *config.MissingFileError
		if errors.As(err, &miss) {
			if err := config.WriteExample(p.Config(), p.FileMode()); err != nil {
				return nil, p, err
			}
			cfg, err = config.LoadWithDefaults(p.Config(), p.KnownHosts())
			if err != nil {
				return nil, p, err
			}
			return cfg, p, nil
		}
		return nil, p, err
	}
	return cfg, p, nil
}
