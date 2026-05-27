package cmd

import (
"fmt"
"os"
"os/exec"

"github.com/spf13/cobra"
"gopkg.in/yaml.v3"

"github.com/HobaiRiku/ssh-tunnel-service/internal/config"
"github.com/HobaiRiku/ssh-tunnel-service/internal/paths"
)

func configCmd() *cobra.Command {
root := &cobra.Command{
Use:   "config",
Short: "Manage service configuration",
}
root.AddCommand(configShowCmd(), configEditCmd(), configPathCmd())
return root
}

func configShowCmd() *cobra.Command {
return &cobra.Command{
Use:   "show",
Short: "Print current configuration as YAML",
RunE: func(_ *cobra.Command, _ []string) error {
p, err := paths.Resolve(rootFlags.Home)
if err != nil {
return err
}
cfg, err := config.Load(p.Config())
if err != nil {
return err
}
return yaml.NewEncoder(os.Stdout).Encode(cfg)
},
}
}

func configEditCmd() *cobra.Command {
return &cobra.Command{
Use:   "edit",
Short: "Open config in $EDITOR",
RunE: func(_ *cobra.Command, _ []string) error {
p, err := paths.Resolve(rootFlags.Home)
if err != nil {
return err
}
editor := os.Getenv("EDITOR")
if editor == "" {
editor = "vi"
}
cmd := exec.Command(editor, p.Config())
cmd.Stdin = os.Stdin
cmd.Stdout = os.Stdout
cmd.Stderr = os.Stderr
return cmd.Run()
},
}
}

func configPathCmd() *cobra.Command {
return &cobra.Command{
Use:   "path",
Short: "Print config file path",
RunE: func(_ *cobra.Command, _ []string) error {
p, err := paths.Resolve(rootFlags.Home)
if err != nil {
return err
}
fmt.Println(p.Config())
return nil
},
}
}
