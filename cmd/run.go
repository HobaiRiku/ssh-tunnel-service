package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"ssh-tunnel-service/internal/service"
)

func runCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Run the service in the foreground",
		RunE: func(c *cobra.Command, _ []string) error {
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()
			return service.Run(ctx, rootFlags.Home, true)
		},
	}
}
