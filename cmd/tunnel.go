package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/HobaiRiku/ssh-tunnel-service/internal/config"
	"github.com/HobaiRiku/ssh-tunnel-service/internal/services"
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
	)
	return root
}

func tunnelListCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tunnels and their status",
		RunE: func(_ *cobra.Command, _ []string) error {
			reg, _, err := registryForCLI(rootFlags.Home)
			if err != nil {
				return err
			}
			tunnels := reg.ListTunnels()
			if jsonOut {
				return json.NewEncoder(os.Stdout).Encode(tunnels)
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tNAME\tDIR\tREMOTE\tBIND\tTARGET\tAUTO")
			for _, t := range tunnels {
				bind := fmt.Sprintf("%s:%d", t.BindAddress, t.BindPort)
				target := fmt.Sprintf("%s:%d", t.TargetHost, t.TargetPort)
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%v\n",
					t.ID, t.Name, t.Direction, t.RemoteID, bind, target, t.AutoStart)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output as JSON")
	return cmd
}

func tunnelAddCmd() *cobra.Command {
	var (
		id, name, remoteID, bindAddr, targetHost, desc, dir string
		bindPort, targetPort                                int
		autoStart                                           bool
	)
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a tunnel definition",
		RunE: func(_ *cobra.Command, _ []string) error {
			reg, _, err := registryForCLI(rootFlags.Home)
			if err != nil {
				return err
			}
			t := config.Tunnel{
				ID:          id,
				Name:        name,
				RemoteID:    remoteID,
				Direction:   config.TunnelDirection(dir),
				BindAddress: bindAddr,
				BindPort:    bindPort,
				TargetHost:  targetHost,
				TargetPort:  targetPort,
				AutoStart:   autoStart,
				Description: desc,
			}
			if err := reg.AddTunnel(t); err != nil {
				return err
			}
			fmt.Printf("Tunnel %q added.\n", id)
			return nil
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "unique ID (required)")
	cmd.Flags().StringVar(&name, "name", "", "display name (required)")
	cmd.Flags().StringVar(&remoteID, "remote", "", "remote ID (required)")
	cmd.Flags().StringVar(&dir, "direction", string(config.DirectionLocal), "-L or -R")
	cmd.Flags().StringVar(&bindAddr, "bind-addr", "127.0.0.1", "bind address")
	cmd.Flags().IntVar(&bindPort, "bind-port", 0, "local port to bind (required)")
	cmd.Flags().StringVar(&targetHost, "target-host", "", "target host (required)")
	cmd.Flags().IntVar(&targetPort, "target-port", 0, "target port (required)")
	cmd.Flags().BoolVar(&autoStart, "auto-start", false, "auto-start with service")
	cmd.Flags().StringVar(&desc, "description", "", "description")
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("remote")
	_ = cmd.MarkFlagRequired("bind-port")
	_ = cmd.MarkFlagRequired("target-host")
	_ = cmd.MarkFlagRequired("target-port")
	return cmd
}

func tunnelUpdateCmd() *cobra.Command {
	var (
		name, remoteID, bindAddr, targetHost, desc, dir string
		bindPort, targetPort                            int
		autoStart                                       bool
	)
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a tunnel definition",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			reg, _, err := registryForCLI(rootFlags.Home)
			if err != nil {
				return err
			}
			current, err2 := reg.GetTunnel(args[0])
			if err2 != nil {
				return err2
			}
			t := current.Tunnel
			if name != "" {
				t.Name = name
			}
			if remoteID != "" {
				t.RemoteID = remoteID
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
			if err := reg.UpdateTunnel(args[0], t); err != nil {
				return err
			}
			fmt.Printf("Tunnel %q updated.\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "display name")
	cmd.Flags().StringVar(&remoteID, "remote", "", "remote ID")
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
		Use:   "rm <id>",
		Short: "Remove a tunnel definition",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			reg, _, err := registryForCLI(rootFlags.Home)
			if err != nil {
				return err
			}
			if err := reg.DeleteTunnel(args[0]); err != nil {
				return err
			}
			fmt.Printf("Tunnel %q removed.\n", args[0])
			return nil
		},
	}
}

func tunnelStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start <id>",
		Short: "Start a tunnel (requires service to be running)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			// When called outside the service context we talk over HTTP.
			// For simplicity we return a helpful message.
			return callHTTPAction(rootFlags.Home, "POST", "/api/tunnels/"+args[0]+"/start")
		},
	}
}

func tunnelStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <id>",
		Short: "Stop a running tunnel",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return callHTTPAction(rootFlags.Home, "POST", "/api/tunnels/"+args[0]+"/stop")
		},
	}
}

// callHTTPAction calls the local API for tunnel start/stop.
func callHTTPAction(home, method, path string) error {
	p, err := registryForCLIPath(home)
	if err != nil {
		return err
	}
	cfg, err := config.LoadWithDefaults(p.Config(), p.KnownHosts())
	if err != nil {
		return err
	}
	_ = services.NewRuntime() // ensure import used
	url := "http://" + cfg.App.HTTPListen + path
	resp, err := httpDo(method, url)
	if err != nil {
		return fmt.Errorf("could not reach service at %s: %w\n(is the service running?)", cfg.App.HTTPListen, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	fmt.Printf("OK (%s %s)\n", method, url)
	return nil
}
