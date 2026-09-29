package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"ssh-tunnel-service/internal/services"
)

func usageCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "usage",
		Short: "Show traffic per tunnel for today, this month and in total",
		Long: "Show how much traffic each tunnel moved today, this month and since it was\n" +
			"first metered. Totals and history are saved to disk by the service, so they\n" +
			"survive restarts. \"down\" is traffic received from the forwarded service,\n" +
			"\"up\" is traffic sent to it.",
		RunE: func(_ *cobra.Command, _ []string) error {
			client, err := newAPIClient(rootFlags.Home)
			if err != nil {
				return err
			}
			var usage services.UsageSummary
			if err := client.request(http.MethodGet, "/api/traffic/usage", nil, &usage); err != nil {
				return err
			}
			if jsonOut {
				return json.NewEncoder(os.Stdout).Encode(usage)
			}
			var tunnels []services.TunnelStatus
			if err := client.request(http.MethodGet, "/api/tunnels", nil, &tunnels); err != nil {
				return err
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "NAME\tTODAY\tTHIS MONTH\tTOTAL")
			for _, ts := range tunnels {
				u, ok := usage.Tunnels[ts.Name]
				if ts.Direct || !ok {
					note := "-"
					if ts.Direct {
						note = "direct"
					}
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", ts.Name, note, note, note)
					continue
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", ts.Name, periodText(u.Today), periodText(u.Month), periodText(u.All))
			}
			t := usage.Total
			fmt.Fprintf(tw, "ALL TUNNELS\t%s\t%s\t%s\n", periodText(t.Today), periodText(t.Month), periodText(t.All))
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output as JSON")
	return cmd
}

// periodText renders a period's traffic as "↓ received ↑ sent".
func periodText(u services.UsageTotals) string {
	return fmt.Sprintf("down %s  up %s", humanBytes(u.DownBytes), humanBytes(u.UpBytes))
}
