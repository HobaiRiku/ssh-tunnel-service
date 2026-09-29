package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"

	"ssh-tunnel-service/internal/services"
)

func topCmd() *cobra.Command {
	var (
		once    bool
		jsonOut bool
		history bool
	)
	cmd := &cobra.Command{
		Use:   "top",
		Short: "Show live per-tunnel and total traffic",
		Long: "Show each tunnel's current speed, open connections and bytes moved,\n" +
			"refreshed every second from the running service. \"up\" is traffic sent\n" +
			"towards the forwarded service, \"down\" is traffic coming back from it.\n\n" +
			"Tunnels created with --direct are not metered and show \"direct\".\n" +
			"--json prints one snapshot of /api/traffic and exits.",
		RunE: func(_ *cobra.Command, _ []string) error {
			client, err := newAPIClient(rootFlags.Home)
			if err != nil {
				return err
			}
			if jsonOut {
				var snap services.TrafficSnapshot
				path := "/api/traffic"
				if history {
					path += "?history=true"
				}
				if err := client.request(http.MethodGet, path, nil, &snap); err != nil {
					return err
				}
				return json.NewEncoder(os.Stdout).Encode(snap)
			}
			if once {
				var snap services.TrafficSnapshot
				if err := client.request(http.MethodGet, "/api/traffic", nil, &snap); err != nil {
					return err
				}
				return renderTop(client, os.Stdout, snap, false)
			}
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()
			return client.watchTraffic(ctx, func(snap services.TrafficSnapshot) error {
				return renderTop(client, os.Stdout, snap, isTerminal(os.Stdout))
			})
		},
	}
	cmd.Flags().BoolVar(&once, "once", false, "print a single frame and exit")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print one traffic snapshot as JSON and exit")
	cmd.Flags().BoolVar(&history, "history", false, "with --json, include the last five minutes of per-second samples")
	return cmd
}

// watchTraffic follows /api/traffic/stream and calls fn for every snapshot
// until ctx is cancelled or the service closes the stream.
func (a *apiClient) watchTraffic(ctx context.Context, fn func(services.TrafficSnapshot) error) error {
	conn, err := a.dialStream(ctx, "/api/traffic/stream?history=false", "traffic stream")
	if err != nil {
		return err
	}
	defer conn.Close()
	go func() {
		<-ctx.Done()
		_ = conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			time.Now().Add(time.Second))
		_ = conn.Close()
	}()
	for {
		var snap services.TrafficSnapshot
		if err := conn.ReadJSON(&snap); err != nil {
			if ctx.Err() != nil ||
				websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return nil
			}
			return err
		}
		if err := fn(snap); err != nil {
			return err
		}
	}
}

// renderTop draws one frame: the aggregate line, then a row per tunnel in
// config order. Tunnel definitions are re-read each frame so state changes and
// newly added tunnels show up live.
func renderTop(client *apiClient, out io.Writer, snap services.TrafficSnapshot, clear bool) error {
	var tunnels []services.TunnelStatus
	if err := client.request(http.MethodGet, "/api/tunnels", nil, &tunnels); err != nil {
		return err
	}
	var b strings.Builder
	if clear {
		b.WriteString("\x1b[H\x1b[2J")
	}
	t := snap.Total
	fmt.Fprintf(&b, "TOTAL  up %s  down %s  conns %d  sent %s  received %s  (%s)\n\n",
		humanRate(t.UpRate), humanRate(t.DownRate), t.ActiveConns,
		humanBytes(t.UpBytes), humanBytes(t.DownBytes), snap.At.Local().Format("15:04:05"))

	tw := tabwriter.NewWriter(&b, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tDIR\tSTATE\tUP\tDOWN\tCONNS\tSENT\tRECEIVED")
	for _, ts := range tunnels {
		c, metered := snap.Tunnels[ts.Name]
		switch {
		case ts.Direct:
			fmt.Fprintf(tw, "%s\t%s\t%s\tdirect\tdirect\t-\t-\t-\n", ts.Name, ts.Direction, ts.State)
		case !metered:
			fmt.Fprintf(tw, "%s\t%s\t%s\t-\t-\t-\t-\t-\n", ts.Name, ts.Direction, ts.State)
		default:
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d\t%s\t%s\n", ts.Name, ts.Direction, ts.State,
				humanRate(c.UpRate), humanRate(c.DownRate), c.ActiveConns,
				humanBytes(c.UpBytes), humanBytes(c.DownBytes))
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if !clear {
		b.WriteString("\n")
	}
	_, err := io.WriteString(out, b.String())
	return err
}

// humanBytes formats a byte count with binary units ("1.5 MiB").
func humanBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for m := n / unit; m >= unit && exp < 5; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

// humanRate formats a bytes-per-second rate ("1.5 MiB/s").
func humanRate(r float64) string {
	if r < 0 {
		r = 0
	}
	return humanBytes(uint64(r+0.5)) + "/s"
}

// isTerminal reports whether f is an interactive terminal, so `top` only
// clears the screen when it is not being piped.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
