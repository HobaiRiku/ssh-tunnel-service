package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func connectCmd() *cobra.Command {
	var (
		show  bool
		clear bool
	)
	cmd := &cobra.Command{
		Use:   "connect [system|user|<home-path>]",
		Short: "Choose which instance the CLI attaches to by default",
		Long: "Set the persistent instance context the CLI attaches to.\n\n" +
			"  connect system          attach to the system service\n" +
			"  connect user            attach to your per-user instance\n" +
			"  connect <home-path>     attach to a custom instance by data root\n" +
			"  connect                 interactively pick system / user / last custom\n" +
			"  connect --show          print the active context and its health\n" +
			"  connect --clear         reset to automatic discovery\n\n" +
			"--home / SSH_TUNNEL_HOME remain one-shot overrides and never change the\n" +
			"saved context.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			switch {
			case show:
				return connectShow()
			case clear:
				return connectClear()
			case len(args) == 1:
				return connectTo(args[0])
			default:
				return connectInteractive()
			}
		},
	}
	cmd.Flags().BoolVar(&show, "show", false, "print the active context and exit")
	cmd.Flags().BoolVar(&clear, "clear", false, "reset to automatic discovery")
	return cmd
}

// connectTo validates and persists a target given as system/user/<path>.
func connectTo(target string) error {
	switch target {
	case scopeSystem, scopeUser:
		if err := verifyScope(target); err != nil {
			return err
		}
		ctx := loadContext()
		ctx.Scope = target
		ctx.Home = ""
		if err := saveContext(ctx); err != nil {
			return err
		}
		fmt.Printf("Connected to the %s instance.\n", target)
		return nil
	default:
		home, err := filepath.Abs(target)
		if err != nil {
			return err
		}
		if err := verifyHome(home); err != nil {
			return err
		}
		ctx := loadContext()
		ctx.Scope = scopeCustom
		ctx.Home = home
		ctx.LastCustom = home
		if err := saveContext(ctx); err != nil {
			return err
		}
		fmt.Printf("Connected to custom instance at %s.\n", home)
		return nil
	}
}

// verifyScope confirms a system/user instance is reachable before pinning it.
func verifyScope(scope string) error {
	epScope, _ := scopeToEndpoint(scope)
	if _, err := clientForScope(epScope); err != nil {
		return fmt.Errorf("cannot connect to the %s instance: %w", scope, err)
	}
	return nil
}

// verifyHome confirms a custom instance answers before pinning it.
func verifyHome(home string) error {
	c, err := clientForHome(home, scopeCustom)
	if err != nil {
		return err
	}
	if !c.healthy() {
		return fmt.Errorf("no healthy instance found at %s", home)
	}
	return nil
}

func connectClear() error {
	ctx := loadContext()
	ctx.Scope = scopeAuto
	ctx.Home = ""
	if err := saveContext(ctx); err != nil {
		return err
	}
	fmt.Println("Context reset to automatic discovery.")
	return nil
}

func connectShow() error {
	ctx := loadContext()
	fmt.Printf("Active context: %s\n", ctx.Scope)
	if ctx.Scope == scopeCustom && ctx.Home != "" {
		fmt.Printf("Home:           %s\n", ctx.Home)
	}
	if ctx.LastCustom != "" {
		fmt.Printf("Last custom:    %s\n", ctx.LastCustom)
	}

	bannerSuppressed = true
	c, err := resolveClient("")
	if err != nil {
		fmt.Printf("Resolved:       (none) — %v\n", err)
		return nil
	}
	status := "unreachable"
	if c.healthy() {
		status = "reachable"
	}
	fmt.Printf("Resolved:       %s · %s (%s)\n", c.scope, c.listen, status)
	return nil
}

// connectInteractive lists the standard targets plus any remembered custom home
// and lets the user pick one.
func connectInteractive() error {
	if !stdinIsTTY() {
		return fmt.Errorf("no target given; specify system, user, or a home path")
	}
	ctx := loadContext()
	options := []string{scopeSystem, scopeUser}
	labels := []string{"system instance", "user instance"}
	if ctx.LastCustom != "" {
		options = append(options, ctx.LastCustom)
		labels = append(labels, "custom: "+ctx.LastCustom)
	}

	fmt.Fprintln(os.Stderr, "Select an instance to connect to:")
	for i, l := range labels {
		fmt.Fprintf(os.Stderr, "  [%d] %s\n", i+1, l)
	}
	fmt.Fprint(os.Stderr, "Choice: ")

	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	n, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || n < 1 || n > len(options) {
		return fmt.Errorf("invalid selection")
	}
	return connectTo(options[n-1])
}
