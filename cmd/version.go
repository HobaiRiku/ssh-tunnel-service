package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"ssh-tunnel-service/internal/version"
)

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Println(version.String())
		},
	}
}
