package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"ssh-tunnel-service/internal/service"
)

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show service status",
		RunE: func(_ *cobra.Command, _ []string) error {
			st, err := service.Status(rootFlags.Home)
			if err != nil {
				return err
			}
			fmt.Println("Service status:", service.StatusString(st))
			return nil
		},
	}
}
