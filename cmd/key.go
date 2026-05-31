package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"ssh-tunnel-service/internal/services"
)

func keyCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "key",
		Short: "Manage stored SSH keys",
	}
	root.AddCommand(keyListCmd(), keyAddCmd(), keyUpdateCmd(), keyRmCmd())
	return root
}

func keyListCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List managed SSH keys",
		RunE: func(_ *cobra.Command, _ []string) error {
			reg, _, err := registryForCLI(rootFlags.Home)
			if err != nil {
				return err
			}
			keys := reg.ListKeys()
			if jsonOut {
				return json.NewEncoder(os.Stdout).Encode(keys)
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tNAME\tFILE\tDESCRIPTION")
			for _, key := range keys {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", key.ID, key.Name, key.File, key.Description)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output as JSON")
	return cmd
}

func keyAddCmd() *cobra.Command {
	var input services.SSHKeyInput
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a managed SSH key",
		RunE: func(_ *cobra.Command, _ []string) error {
			reg, _, err := registryForCLI(rootFlags.Home)
			if err != nil {
				return err
			}
			if err := loadKeyMaterial(&input); err != nil {
				return err
			}
			if err := reg.AddKey(input); err != nil {
				return err
			}
			fmt.Printf("Key %q added.\n", input.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&input.ID, "id", "", "unique ID (required)")
	cmd.Flags().StringVar(&input.Name, "name", "", "display name (required)")
	cmd.Flags().StringVar(&input.Description, "description", "", "description")
	cmd.Flags().StringVar(&input.FileName, "file-name", "", "stored file name override")
	cmd.Flags().StringVar(&input.PrivateKey, "private-key", "", "private key content")
	cmd.Flags().StringVar(&input.SourcePath, "source", "", "copy key material from an existing file")
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func keyUpdateCmd() *cobra.Command {
	var input services.SSHKeyInput
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a managed SSH key",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			reg, _, err := registryForCLI(rootFlags.Home)
			if err != nil {
				return err
			}
			if err := loadKeyMaterial(&input); err != nil {
				return err
			}
			if err := reg.UpdateKey(args[0], input); err != nil {
				return err
			}
			fmt.Printf("Key %q updated.\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&input.Name, "name", "", "display name")
	cmd.Flags().StringVar(&input.Description, "description", "", "description")
	cmd.Flags().StringVar(&input.FileName, "file-name", "", "stored file name override")
	cmd.Flags().StringVar(&input.PrivateKey, "private-key", "", "private key content")
	cmd.Flags().StringVar(&input.SourcePath, "source", "", "copy key material from an existing file")
	return cmd
}

func keyRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <id>",
		Short: "Remove a managed SSH key",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			reg, _, err := registryForCLI(rootFlags.Home)
			if err != nil {
				return err
			}
			if err := reg.DeleteKey(args[0]); err != nil {
				return err
			}
			fmt.Printf("Key %q removed.\n", args[0])
			return nil
		},
	}
}

func loadKeyMaterial(input *services.SSHKeyInput) error {
	if input == nil || input.SourcePath == "" {
		return nil
	}
	if input.PrivateKey != "" {
		return fmt.Errorf("provide either --private-key or --source, not both")
	}
	content, err := os.ReadFile(input.SourcePath)
	if err != nil {
		return fmt.Errorf("read key source %s: %w", input.SourcePath, err)
	}
	input.PrivateKey = string(content)
	return nil
}
