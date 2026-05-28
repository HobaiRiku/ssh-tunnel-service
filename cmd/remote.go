package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/HobaiRiku/ssh-tunnel-service/internal/config"
	"github.com/HobaiRiku/ssh-tunnel-service/internal/paths"
	"github.com/HobaiRiku/ssh-tunnel-service/internal/services"
)

func remoteCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "remote",
		Short: "Manage remote SSH targets",
	}
	root.AddCommand(remoteListCmd(), remoteAddCmd(), remoteUpdateCmd(), remoteRmCmd())
	return root
}

func remoteListCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List remotes",
		RunE: func(_ *cobra.Command, _ []string) error {
			reg, _, err := registryForCLI(rootFlags.Home)
			if err != nil {
				return err
			}
			remotes := reg.ListRemotes()
			if jsonOut {
				return json.NewEncoder(os.Stdout).Encode(remotes)
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tNAME\tHOST\tPORT\tUSER\tDESCRIPTION")
			for _, r := range remotes {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\t%s\n", r.ID, r.Name, r.Host, r.Port, r.User, r.Description)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output as JSON")
	return cmd
}

func remoteAddCmd() *cobra.Command {
	var (
		id, name, host, user, desc string
		port                       int
	)
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a remote",
		RunE: func(_ *cobra.Command, _ []string) error {
			reg, _, err := registryForCLI(rootFlags.Home)
			if err != nil {
				return err
			}
			r := config.Remote{ID: id, Name: name, Host: host, Port: port, User: user, Description: desc}
			if err := reg.AddRemote(r); err != nil {
				return err
			}
			fmt.Printf("Remote %q (%s) added.\n", id, name)
			return nil
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "unique ID (required)")
	cmd.Flags().StringVar(&name, "name", "", "display name (required)")
	cmd.Flags().StringVar(&host, "host", "", "SSH host (required)")
	cmd.Flags().IntVar(&port, "port", 22, "SSH port")
	cmd.Flags().StringVar(&user, "user", "", "SSH user")
	cmd.Flags().StringVar(&desc, "description", "", "description")
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("host")
	return cmd
}

func remoteUpdateCmd() *cobra.Command {
	var (
		name, host, user, desc, portStr string
	)
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a remote",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			reg, _, err := registryForCLI(rootFlags.Home)
			if err != nil {
				return err
			}
			current, err2 := reg.GetRemote(args[0])
			if err2 != nil {
				return err2
			}
			r := current
			if name != "" {
				r.Name = name
			}
			if host != "" {
				r.Host = host
			}
			if portStr != "" {
				p, err := strconv.Atoi(portStr)
				if err != nil {
					return fmt.Errorf("invalid port: %w", err)
				}
				r.Port = p
			}
			if user != "" {
				r.User = user
			}
			if desc != "" {
				r.Description = desc
			}
			if err := reg.UpdateRemote(args[0], r); err != nil {
				return err
			}
			fmt.Printf("Remote %q updated.\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "display name")
	cmd.Flags().StringVar(&host, "host", "", "SSH host")
	cmd.Flags().StringVar(&portStr, "port", "", "SSH port")
	cmd.Flags().StringVar(&user, "user", "", "SSH user")
	cmd.Flags().StringVar(&desc, "description", "", "description")
	return cmd
}

func remoteRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <id>",
		Short: "Remove a remote",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			reg, _, err := registryForCLI(rootFlags.Home)
			if err != nil {
				return err
			}
			if err := reg.DeleteRemote(args[0]); err != nil {
				return err
			}
			fmt.Printf("Remote %q removed.\n", args[0])
			return nil
		},
	}
}

// registryForCLI loads config and builds a Registry for CLI commands that read/write config.
func registryForCLI(home string) (*services.Registry, paths.Paths, error) {
	p, err := paths.Resolve(home)
	if err != nil {
		return nil, paths.Paths{}, err
	}
	if err := p.EnsureTree(); err != nil {
		return nil, paths.Paths{}, err
	}
	cfg, err := config.Load(p.Config())
	if err != nil {
		var miss *config.MissingFileError
		if errors.As(err, &miss) {
			if err := config.WriteExample(p.Config(), p.FileMode()); err != nil {
				return nil, p, err
			}
			cfg, err = config.Load(p.Config())
			if err != nil {
				return nil, p, err
			}
		} else {
			return nil, p, err
		}
	}
	config.ApplyDefaults(cfg, p.KnownHosts())
	if err := config.Validate(cfg); err != nil {
		return nil, p, err
	}
	rt := services.NewRuntime()
	return services.New(cfg, p, rt), p, nil
}
