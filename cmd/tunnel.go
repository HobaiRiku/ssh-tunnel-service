package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"ssh-tunnel-service/internal/config"
	"ssh-tunnel-service/internal/services"
)

func tunnelCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "tunnel",
		Short: "Manage SSH tunnel definitions",
	}
	root.AddCommand(
		tunnelListCmd(),
		tunnelAddCmd(),
		tunnelUpdateCmd(),
		tunnelRmCmd(),
		tunnelStartCmd(),
		tunnelStopCmd(),
		tunnelRestartCmd(),
	)
	return root
}

func tunnelListCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tunnels and their live status",
		RunE: func(_ *cobra.Command, _ []string) error {
			client, err := newAPIClient(rootFlags.Home)
			if err != nil {
				return err
			}
			var tunnels []services.TunnelStatus
			if err := client.request(http.MethodGet, "/api/tunnels", nil, &tunnels); err != nil {
				return err
			}
			if jsonOut {
				return json.NewEncoder(os.Stdout).Encode(tunnels)
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "NAME\tDIR\tREMOTE\tBIND\tTARGET\tAUTO\tSTATE\tPID")
			for _, t := range tunnels {
				bind := fmt.Sprintf("%s:%d", t.BindAddress, t.BindPort)
				target := fmt.Sprintf("%s:%d", t.TargetHost, t.TargetPort)
				pid := ""
				if t.PID != 0 {
					pid = fmt.Sprintf("%d", t.PID)
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%v\t%s\t%s\n",
					t.Name, t.Direction, t.Remote, bind, target, t.AutoStart, t.State, pid)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output as JSON")
	return cmd
}

func tunnelAddCmd() *cobra.Command {
	var (
		name, remote, bindAddr, targetHost, desc, dir string
		bindPort, targetPort                          int
		autoStart                                     bool
	)
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a tunnel definition",
		RunE: func(_ *cobra.Command, _ []string) error {
			client, err := newAPIClient(rootFlags.Home)
			if err != nil {
				return err
			}
			t := config.Tunnel{
				Name:        name,
				Remote:      remote,
				Direction:   config.TunnelDirection(dir),
				BindAddress: bindAddr,
				BindPort:    bindPort,
				TargetHost:  targetHost,
				TargetPort:  targetPort,
				AutoStart:   autoStart,
				Description: desc,
			}
			if err := client.request(http.MethodPost, "/api/tunnels", t, nil); err != nil {
				return err
			}
			fmt.Printf("Tunnel %q added.\n", name)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "unique name (required)")
	cmd.Flags().StringVar(&remote, "remote", "", "remote name (required)")
	cmd.Flags().StringVar(&dir, "direction", string(config.DirectionLocal), "-L or -R")
	cmd.Flags().StringVar(&bindAddr, "bind-addr", "127.0.0.1", "bind address")
	cmd.Flags().IntVar(&bindPort, "bind-port", 0, "local port to bind (required)")
	cmd.Flags().StringVar(&targetHost, "target-host", "", "target host (required)")
	cmd.Flags().IntVar(&targetPort, "target-port", 0, "target port (required)")
	cmd.Flags().BoolVar(&autoStart, "auto-start", false, "auto-start with service and keep alive")
	cmd.Flags().StringVar(&desc, "description", "", "description")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("remote")
	_ = cmd.MarkFlagRequired("bind-port")
	_ = cmd.MarkFlagRequired("target-host")
	_ = cmd.MarkFlagRequired("target-port")
	return cmd
}

func tunnelUpdateCmd() *cobra.Command {
	var (
		name, remote, bindAddr, targetHost, desc, dir string
		bindPort, targetPort                          int
		autoStart                                     bool
	)
	cmd := &cobra.Command{
		Use:   "update <name>",
		Short: "Update a tunnel definition (use --name to rename)",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			client, err := newAPIClient(rootFlags.Home)
			if err != nil {
				return err
			}
			// The update endpoint replaces the whole tunnel, so fetch the current
			// definition and overlay only the flags the user supplied.
			var t config.Tunnel
			if err := client.request(http.MethodGet, "/api/tunnels/"+url.PathEscape(args[0]), nil, &t); err != nil {
				return err
			}
			if name != "" {
				t.Name = name
			}
			if remote != "" {
				t.Remote = remote
			}
			if dir != "" {
				t.Direction = config.TunnelDirection(dir)
			}
			if bindAddr != "" {
				t.BindAddress = bindAddr
			}
			if bindPort != 0 {
				t.BindPort = bindPort
			}
			if targetHost != "" {
				t.TargetHost = targetHost
			}
			if targetPort != 0 {
				t.TargetPort = targetPort
			}
			if c.Flags().Changed("auto-start") {
				t.AutoStart = autoStart
			}
			if desc != "" {
				t.Description = desc
			}
			if err := client.request(http.MethodPut, "/api/tunnels/"+url.PathEscape(args[0]), t, nil); err != nil {
				return err
			}
			fmt.Printf("Tunnel %q updated.\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "rename the tunnel")
	cmd.Flags().StringVar(&remote, "remote", "", "remote name")
	cmd.Flags().StringVar(&dir, "direction", "", "-L or -R")
	cmd.Flags().StringVar(&bindAddr, "bind-addr", "", "bind address")
	cmd.Flags().IntVar(&bindPort, "bind-port", 0, "local bind port")
	cmd.Flags().StringVar(&targetHost, "target-host", "", "target host")
	cmd.Flags().IntVar(&targetPort, "target-port", 0, "target port")
	cmd.Flags().BoolVar(&autoStart, "auto-start", false, "auto-start with service")
	cmd.Flags().StringVar(&desc, "description", "", "description")
	return cmd
}

func tunnelRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a tunnel definition",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			client, err := newAPIClient(rootFlags.Home)
			if err != nil {
				return err
			}
			if err := client.request(http.MethodDelete, "/api/tunnels/"+url.PathEscape(args[0]), nil, nil); err != nil {
				return err
			}
			fmt.Printf("Tunnel %q removed.\n", args[0])
			return nil
		},
	}
}

func tunnelStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start <name>",
		Short: "Start a tunnel (requires the service to be running)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return tunnelAction(args[0], "start", "started")
		},
	}
}

func tunnelStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <name>",
		Short: "Stop a running tunnel",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return tunnelAction(args[0], "stop", "stopped")
		},
	}
}

func tunnelRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart <name>",
		Short: "Restart a tunnel (stop if running, then start) via the running service",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return tunnelAction(args[0], "restart", "restarted")
		},
	}
}

// tunnelAction invokes a lifecycle endpoint (start/stop/restart) on the running
// service and reports the outcome in plain language — never the underlying HTTP
// request details.
func tunnelAction(name, action, done string) error {
	client, err := newAPIClient(rootFlags.Home)
	if err != nil {
		return err
	}
	if err := client.request(http.MethodPost, "/api/tunnels/"+url.PathEscape(name)+"/"+action, nil, nil); err != nil {
		return err
	}
	fmt.Printf("Tunnel %q %s.\n", name, done)
	return nil
}
