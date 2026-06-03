package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"ssh-tunnel-service/internal/service"
)

func installCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install the system service",
		RunE: func(_ *cobra.Command, _ []string) error {
			if handled, err := ensurePrivileged(); handled {
				return err
			}
			if err := service.Install(rootFlags.Home); err != nil {
				return err
			}
			fmt.Println("Service installed successfully.")
			return nil
		},
	}
}
