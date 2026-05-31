// Package cmd hosts the cobra command tree.
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"ssh-tunnel-service/internal/version"
)

var rootFlags struct {
	Home string
}

// Root returns the wired cobra root command.
func Root() *cobra.Command {
	root := &cobra.Command{
		Use:           "ssh-tunnel",
		Short:         "Manage SSH -L/-R port-forwarding tunnels as a service",
		SilenceUsage:  true,
		SilenceErrors: false,
		Run: func(c *cobra.Command, _ []string) {
			if show, _ := c.Flags().GetBool("version"); show {
				fmt.Println(version.String())
				return
			}
			_ = c.Help()
		},
	}
	root.PersistentFlags().StringVar(&rootFlags.Home, "home", "",
		"override SSH_TUNNEL_HOME (default $HOME/.ssh-tunnel-service)")
	root.Flags().BoolP("version", "v", false, "print version and exit")

	root.AddCommand(
		runCmd(),
		installCmd(),
		uninstallCmd(),
		startCmd(),
		stopCmd(),
		statusCmd(),
		tailCmd(),
		remoteCmd(),
		keyCmd(),
		tunnelCmd(),
		configCmd(),
		versionCmd(),
	)
	return root
}

// Execute runs the root command.
func Execute() error { return Root().Execute() }
