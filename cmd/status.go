package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

// instancePayload mirrors GET /api/instance.
type instancePayload struct {
	Scope         string  `json:"scope"`
	Home          string  `json:"home"`
	Address       string  `json:"address"`
	PID           int     `json:"pid"`
	Version       string  `json:"version"`
	UptimeSeconds float64 `json:"uptime_seconds"`
}

func statusCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show the active instance's status",
		Long: "Report the running instance the CLI is attached to, read-only over\n" +
			"the loopback API — no elevation required.",
		RunE: func(_ *cobra.Command, _ []string) error {
			// Suppress the banner; status prints the identity itself.
			bannerSuppressed = true
			client, err := resolveClient(rootFlags.Home)
			if err != nil {
				return err
			}
			var info instancePayload
			if err := client.request(http.MethodGet, "/api/instance", nil, &info); err != nil {
				if jsonOut {
					return json.NewEncoder(os.Stdout).Encode(map[string]any{
						"running": false,
						"address": client.listen,
					})
				}
				fmt.Printf("Instance at %s: not running\n", client.listen)
				return nil
			}
			if jsonOut {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"running":        true,
					"scope":          info.Scope,
					"home":           info.Home,
					"address":        info.Address,
					"pid":            info.PID,
					"version":        info.Version,
					"uptime_seconds": info.UptimeSeconds,
				})
			}
			fmt.Printf("Status:   running\n")
			fmt.Printf("Scope:    %s\n", info.Scope)
			fmt.Printf("Address:  %s\n", client.listen)
			fmt.Printf("Home:     %s\n", info.Home)
			fmt.Printf("PID:      %d\n", info.PID)
			fmt.Printf("Version:  %s\n", info.Version)
			fmt.Printf("Uptime:   %s\n", formatUptime(info.UptimeSeconds))
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output as JSON")
	return cmd
}

func formatUptime(seconds float64) string {
	s := int(seconds)
	d, s := s/86400, s%86400
	h, s := s/3600, s%3600
	m, s := s/60, s%60
	switch {
	case d > 0:
		return fmt.Sprintf("%dd %dh %dm", d, h, m)
	case h > 0:
		return fmt.Sprintf("%dh %dm", h, m)
	case m > 0:
		return fmt.Sprintf("%dm %ds", m, s)
	default:
		return fmt.Sprintf("%ds", s)
	}
}
