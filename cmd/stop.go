package cmd

import (
"fmt"

"github.com/spf13/cobra"

"github.com/HobaiRiku/ssh-tunnel-service/internal/service"
)

func stopCmd() *cobra.Command {
return &cobra.Command{
Use:   "stop",
Short: "Stop the installed service",
RunE: func(_ *cobra.Command, _ []string) error {
if err := service.Stop(rootFlags.Home); err != nil {
return err
}
fmt.Println("Service stopped.")
return nil
},
}
}
