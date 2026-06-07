package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"ssh-tunnel-service/internal/service"
)

func uninstallCmd() *cobra.Command {
	var userScope bool
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall the service (use --user for a per-user install)",
		RunE: func(_ *cobra.Command, _ []string) error {
			// System scope needs root; user scope runs as the invoking user.
			if !userScope {
				if handled, err := ensurePrivileged(); handled {
					return err
				}
			}
			if err := service.Uninstall(rootFlags.Home, userScope); err != nil {
				return err
			}
			fmt.Println("Service uninstalled successfully.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&userScope, "user", false, "uninstall the per-user service")
	return cmd
}
