package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"ssh-tunnel-service/internal/elevate"
	"ssh-tunnel-service/internal/skill"
	"ssh-tunnel-service/internal/version"
)

type skillFlags struct {
	Targets      []string
	Project      bool
	Dir          string
	Plugin       bool
	AgentsMD     bool
	AgentsMDPath string
	Force        bool
	DryRun       bool
}

func skillCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "skill",
		Short: "Install the ssh-tunnel Agent Skill into AI coding agents",
		Long: "Install the ssh-tunnel Agent Skill so AI coding agents can manage tunnels.\n\n" +
			"The skill is a single SKILL.md in the shared Agent Skills format, but every\n" +
			"client discovers skills in its own root, so installing means copying it into\n" +
			"each one: ~/.claude/skills for Claude Code, ~/.agents/skills for Codex.\n" +
			"With no --target, every client whose root already exists is installed into.",
	}
	root.AddCommand(skillInstallCmd(), skillUninstallCmd(), skillPrintCmd())
	return root
}

// bindSkillFlags wires the destination flags shared by install and uninstall.
func bindSkillFlags(cmd *cobra.Command, f *skillFlags) {
	cmd.Flags().StringSliceVar(&f.Targets, "target", nil,
		"agent clients to install into: claude, codex, or all (default: auto-detect)")
	cmd.Flags().BoolVar(&f.Project, "project", false,
		"install into the current repository instead of the user's home")
	cmd.Flags().StringVar(&f.Dir, "dir", "",
		"write to this directory instead of a known client root")
	cmd.Flags().BoolVar(&f.Plugin, "plugin", false,
		"with --dir, write the whole Agent Plugin (plugin.json + skills/) instead of the bare skill")
	// Two flags rather than one optional-value flag: pflag only accepts
	// `--flag=value` for those, so `--agents-md ./AGENTS.md` would silently
	// fall back to the default path and write to the wrong file.
	cmd.Flags().BoolVar(&f.AgentsMD, "agents-md", false,
		"also inject a summary block into the repository root AGENTS.md")
	cmd.Flags().StringVar(&f.AgentsMDPath, "agents-md-path", "",
		"like --agents-md, but write to this file instead of the repository root")
	cmd.Flags().BoolVar(&f.Force, "force", false,
		"overwrite a destination not installed by ssh-tunnel, or with local edits")
	cmd.Flags().BoolVar(&f.DryRun, "dry-run", false,
		"report what would change without writing anything")
}

func skillInstallCmd() *cobra.Command {
	var f skillFlags
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the Agent Skill into detected agent clients",
		RunE: func(c *cobra.Command, _ []string) error {
			return runSkill(c, &f, false)
		},
	}
	bindSkillFlags(cmd, &f)
	return cmd
}

func skillUninstallCmd() *cobra.Command {
	var f skillFlags
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the Agent Skill from agent clients",
		RunE: func(c *cobra.Command, _ []string) error {
			return runSkill(c, &f, true)
		},
	}
	bindSkillFlags(cmd, &f)
	return cmd
}

func skillPrintCmd() *cobra.Command {
	var reference bool
	cmd := &cobra.Command{
		Use:   "print",
		Short: "Print the skill to stdout, for clients install does not know about",
		RunE: func(c *cobra.Command, _ []string) error {
			name := "SKILL.md"
			if reference {
				name = "reference.md"
			}
			data, err := skill.File(name)
			if err != nil {
				return err
			}
			_, err = c.OutOrStdout().Write(data)
			return err
		},
	}
	cmd.Flags().BoolVar(&reference, "reference", false, "print reference.md instead of SKILL.md")
	return cmd
}

// runSkill performs an install or uninstall across every requested destination.
func runSkill(c *cobra.Command, f *skillFlags, remove bool) error {
	if err := refuseElevated(); err != nil {
		return err
	}
	if f.Plugin && f.Dir == "" {
		return fmt.Errorf("--plugin requires --dir: no known client root reads plugin.json out of its skills directory")
	}

	payload, err := skillPayload(f.Plugin)
	if err != nil {
		return err
	}
	dests, err := skillDestinations(f)
	if err != nil {
		return err
	}

	opts := skill.Options{Version: version.Version, Force: f.Force, DryRun: f.DryRun}
	out := c.OutOrStdout()
	if f.DryRun {
		fmt.Fprintln(out, "dry run — nothing will be written")
	}

	for _, d := range dests {
		outcome, err := applySkill(d, payload, opts, remove)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "%-14s %s  %s\n", outcome, d.path, d.label)
	}

	if f.AgentsMD || f.AgentsMDPath != "" {
		path := f.AgentsMDPath
		if path == "" {
			if path, err = skill.AgentsPath(); err != nil {
				return err
			}
		}
		outcome, err := applyAgents(path, opts, remove)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "%-14s %s  AGENTS.md block\n", outcome, path)
	}
	return nil
}

// destination is one resolved place to write, with a label for reporting.
type destination struct {
	path  string
	label string
}

func skillPayload(plugin bool) (fs.FS, error) {
	if plugin {
		return skill.PluginFS()
	}
	return skill.SkillFS()
}

// skillDestinations resolves --dir, or the requested/auto-detected clients.
func skillDestinations(f *skillFlags) ([]destination, error) {
	if f.Dir != "" {
		if len(f.Targets) > 0 || f.Project {
			return nil, fmt.Errorf("--dir names the destination directly; drop --target/--project")
		}
		label := "explicit directory"
		if f.Plugin {
			label = "explicit directory (full plugin)"
		}
		return []destination{{path: f.Dir, label: label}}, nil
	}

	clients, err := parseTargets(f.Targets)
	if err != nil {
		return nil, err
	}
	scope := skill.ScopeUser
	if f.Project {
		scope = skill.ScopeProject
	}
	targets, err := skill.ResolveTargets(clients, scope)
	if err != nil {
		return nil, err
	}
	dests := make([]destination, 0, len(targets))
	for _, t := range targets {
		dests = append(dests, destination{path: t.Dir, label: t.String()})
	}
	return dests, nil
}

// parseTargets maps --target values to clients; "all" expands to every client
// this command knows, whether or not it is installed on the machine.
func parseTargets(values []string) ([]skill.Client, error) {
	var out []skill.Client
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if v == "all" {
			return skill.Clients, nil
		}
		c, err := skill.ParseClient(v)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

func applySkill(d destination, payload fs.FS, opts skill.Options, remove bool) (skill.Outcome, error) {
	if remove {
		return skill.Uninstall(d.path, payload, opts)
	}
	return skill.Install(d.path, payload, opts)
}

func applyAgents(path string, opts skill.Options, remove bool) (skill.Outcome, error) {
	if remove {
		return skill.RemoveAgents(path, opts)
	}
	return skill.InjectAgents(path, opts)
}

// refuseElevated blocks the command under sudo.
//
// Unlike the service-control commands this one must never elevate: it writes
// into the invoking user's home, and under sudo os.UserHomeDir() resolves to
// root's. That failure is silent — the files land somewhere the user's agent
// never looks — so it is caught up front rather than reported afterwards.
//
// A plain root session (a container, a root-only machine) is left alone: there
// the root home *is* the right destination, and SUDO_USER is what distinguishes
// the two cases.
func refuseElevated() error {
	if !elevate.IsElevated() {
		return nil
	}
	user := os.Getenv("SUDO_USER")
	if user == "" {
		return nil
	}
	return fmt.Errorf(
		"do not run this under sudo: the skill would be installed into root's home, not %s's; re-run without sudo, or pass --dir to choose the destination explicitly",
		user)
}
