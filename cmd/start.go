package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"ssh-tunnel-service/internal/service"
)

func startCmd() *cobra.Command {
	var userScope bool
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the installed service (use --user for a per-user install)",
		RunE: func(_ *cobra.Command, _ []string) error {
			if !userScope {
				if handled, err := ensurePrivileged(); handled {
					return err
				}
			}
			if err := service.Start(rootFlags.Home, userScope); err != nil {
				return err
			}
			fmt.Println("Service started.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&userScope, "user", false, "start the per-user service")
	return cmd
}
