package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"ssh-tunnel-service/internal/config"
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
			client, err := newAPIClient(rootFlags.Home)
			if err != nil {
				return err
			}
			var remotes []config.Remote
			if err := client.request(http.MethodGet, "/api/remotes", nil, &remotes); err != nil {
				return err
			}
			if jsonOut {
				return json.NewEncoder(os.Stdout).Encode(remotes)
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "NAME\tHOST\tPORT\tUSER\tKEY\tDESCRIPTION")
			for _, r := range remotes {
				fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\t%s\n", r.Name, r.Host, r.Port, r.User, r.Key, r.Description)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output as JSON")
	return cmd
}

func remoteAddCmd() *cobra.Command {
	var (
		name, host, user, key, desc string
		port                        int
	)
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a remote",
		RunE: func(_ *cobra.Command, _ []string) error {
			client, err := newAPIClient(rootFlags.Home)
			if err != nil {
				return err
			}
			r := config.Remote{Name: name, Host: host, Port: port, User: user, Key: key, Description: desc}
			if err := client.request(http.MethodPost, "/api/remotes", r, nil); err != nil {
				return err
			}
			fmt.Printf("Remote %q added.\n", name)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "unique name (required)")
	cmd.Flags().StringVar(&host, "host", "", "SSH host (required)")
	cmd.Flags().IntVar(&port, "port", 22, "SSH port")
	cmd.Flags().StringVar(&user, "user", "", "SSH user")
	cmd.Flags().StringVar(&key, "key", "", "managed key name")
	cmd.Flags().StringVar(&desc, "description", "", "description")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("host")
	return cmd
}

func remoteUpdateCmd() *cobra.Command {
	var (
		name, host, user, key, desc, portStr string
	)
	cmd := &cobra.Command{
		Use:   "update <name>",
		Short: "Update a remote (use --name to rename)",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			client, err := newAPIClient(rootFlags.Home)
			if err != nil {
				return err
			}
			var r config.Remote
			if err := client.request(http.MethodGet, "/api/remotes/"+url.PathEscape(args[0]), nil, &r); err != nil {
				return err
			}
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
			if c.Flags().Changed("key") {
				r.Key = key
			}
			if desc != "" {
				r.Description = desc
			}
			if err := client.request(http.MethodPut, "/api/remotes/"+url.PathEscape(args[0]), r, nil); err != nil {
				return err
			}
			fmt.Printf("Remote %q updated.\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "rename the remote")
	cmd.Flags().StringVar(&host, "host", "", "SSH host")
	cmd.Flags().StringVar(&portStr, "port", "", "SSH port")
	cmd.Flags().StringVar(&user, "user", "", "SSH user")
	cmd.Flags().StringVar(&key, "key", "", "managed key name (empty to clear)")
	cmd.Flags().StringVar(&desc, "description", "", "description")
	return cmd
}

func remoteRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a remote",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			client, err := newAPIClient(rootFlags.Home)
			if err != nil {
				return err
			}
			if err := client.request(http.MethodDelete, "/api/remotes/"+url.PathEscape(args[0]), nil, nil); err != nil {
				return err
			}
			fmt.Printf("Remote %q removed.\n", args[0])
			return nil
		},
	}
}
