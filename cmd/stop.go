package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"ssh-tunnel-service/internal/service"
)

func stopCmd() *cobra.Command {
	var userScope bool
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the installed service (use --user for a per-user install)",
		RunE: func(_ *cobra.Command, _ []string) error {
			if !userScope {
				if handled, err := ensurePrivileged(); handled {
					return err
				}
			}
			if err := service.Stop(rootFlags.Home, userScope); err != nil {
				return err
			}
			fmt.Println("Service stopped.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&userScope, "user", false, "stop the per-user service")
	return cmd
}
