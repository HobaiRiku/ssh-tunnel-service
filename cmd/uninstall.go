package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"ssh-tunnel-service/internal/service"
)

func uninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall the system service",
		RunE: func(_ *cobra.Command, _ []string) error {
			if handled, err := ensurePrivileged(); handled {
				return err
			}
			if err := service.Uninstall(rootFlags.Home); err != nil {
				return err
			}
			fmt.Println("Service uninstalled successfully.")
			return nil
		},
	}
}
