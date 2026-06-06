package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

func tailCmd() *cobra.Command {
	var (
		lines  int
		follow bool
	)

	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Tail the attached instance's log in real time",
		Long: "Stream the running service's log over the loopback API (WebSocket).\n" +
			"No filesystem access to the data root is required, so it works against\n" +
			"the root-owned system service as a normal user.",
		RunE: func(_ *cobra.Command, _ []string) error {
			client, err := newAPIClient(rootFlags.Home)
			if err != nil {
				return err
			}
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()
			return client.streamLogs(ctx, lines, follow, os.Stdout)
		},
	}
	cmd.Flags().IntVarP(&lines, "lines", "n", 50, "number of recent log lines to print first")
	cmd.Flags().BoolVarP(&follow, "follow", "f", true, "follow appended log output")
	return cmd
}
