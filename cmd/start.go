package cmd

import (
"fmt"

"github.com/spf13/cobra"

"github.com/HobaiRiku/ssh-tunnel-service/internal/service"
)

func startCmd() *cobra.Command {
return &cobra.Command{
Use:   "start",
Short: "Start the installed service",
RunE: func(_ *cobra.Command, _ []string) error {
if err := service.Start(rootFlags.Home); err != nil {
return err
}
fmt.Println("Service started.")
return nil
},
}
}
