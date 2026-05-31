package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"ssh-tunnel-service/internal/paths"
)

func tailCmd() *cobra.Command {
	var (
		lines  int
		follow bool
	)

	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Tail the current service log",
		RunE: func(_ *cobra.Command, _ []string) error {
			p, err := paths.Resolve(rootFlags.Home)
			if err != nil {
				return err
			}
			ctx := context.Background()
			if follow {
				var cancel context.CancelFunc
				ctx, cancel = signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
				defer cancel()
			}
			return tailLog(ctx, p.LogFile(), lines, follow, os.Stdout)
		},
	}
	cmd.Flags().IntVarP(&lines, "lines", "n", 50, "number of recent log lines to print first")
	cmd.Flags().BoolVarP(&follow, "follow", "f", true, "follow appended log output")
	return cmd
}

func tailLog(ctx context.Context, path string, lines int, follow bool, out io.Writer) error {
	if lines < 0 {
		return fmt.Errorf("lines must be >= 0")
	}
	if err := printLastLines(path, lines, out); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if !follow {
		return nil
	}
	return followLog(ctx, path, out)
}

func printLastLines(path string, lines int, out io.Writer) error {
	if lines == 0 {
		return nil
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	var ring []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		ring = append(ring, scanner.Text())
		if len(ring) > lines {
			ring = ring[1:]
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	for _, line := range ring {
		if _, err := fmt.Fprintln(out, line); err != nil {
			return err
		}
	}
	return nil
}

func followLog(ctx context.Context, path string, out io.Writer) error {
	var offset int64
	if info, err := os.Stat(path); err == nil {
		offset = info.Size()
	}

	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}

		file, err := os.Open(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				offset = 0
				continue
			}
			return err
		}

		info, err := file.Stat()
		if err != nil {
			_ = file.Close()
			return err
		}
		if info.Size() < offset {
			offset = 0
		}
		if _, err := file.Seek(offset, io.SeekStart); err != nil {
			_ = file.Close()
			return err
		}
		n, err := io.Copy(out, file)
		_ = file.Close()
		if err != nil {
			return err
		}
		offset += n
	}
}
