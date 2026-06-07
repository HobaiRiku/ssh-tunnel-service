package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"ssh-tunnel-service/internal/config"
	"ssh-tunnel-service/internal/services"
)

func keyCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "key",
		Short: "Manage stored SSH keys",
	}
	root.AddCommand(keyListCmd(), keyAddCmd(), keyUpdateCmd(), keyRmCmd(), keySetDefaultCmd(), keyPubCmd())
	return root
}

// keyPubCmd prints a managed key's public key (authorized_keys line) so it can
// be copied to a target server's ~/.ssh/authorized_keys.
func keyPubCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pub <name>",
		Short: "Print a key's public key (authorized_keys line)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			client, err := newAPIClient(rootFlags.Home)
			if err != nil {
				return err
			}
			var key config.SSHKey
			if err := client.request(http.MethodGet, "/api/keys/"+url.PathEscape(args[0]), nil, &key); err != nil {
				return err
			}
			if key.Public == "" {
				return fmt.Errorf("key %q has no public key available", args[0])
			}
			fmt.Println(key.Public)
			return nil
		},
	}
}

func keySetDefaultCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-default <name>",
		Short: "Designate a key as the system default for unbound tunnels",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			client, err := newAPIClient(rootFlags.Home)
			if err != nil {
				return err
			}
			if err := client.request(http.MethodPut, "/api/keys/"+url.PathEscape(args[0])+"/default", nil, nil); err != nil {
				return err
			}
			fmt.Printf("Key %q is now the system default.\n", args[0])
			return nil
		},
	}
}

func keyListCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List managed SSH keys",
		RunE: func(_ *cobra.Command, _ []string) error {
			client, err := newAPIClient(rootFlags.Home)
			if err != nil {
				return err
			}
			var keys []config.SSHKey
			if err := client.request(http.MethodGet, "/api/keys", nil, &keys); err != nil {
				return err
			}
			if jsonOut {
				return json.NewEncoder(os.Stdout).Encode(keys)
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "NAME\tFILE\tDESCRIPTION")
			for _, key := range keys {
				fmt.Fprintf(tw, "%s\t%s\t%s\n", key.Name, key.File, key.Description)
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
			client, err := newAPIClient(rootFlags.Home)
			if err != nil {
				return err
			}
			if err := loadKeyMaterial(&input); err != nil {
				return err
			}
			if err := client.request(http.MethodPost, "/api/keys", input, nil); err != nil {
				return err
			}
			fmt.Printf("Key %q added.\n", input.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&input.Name, "name", "", "unique name (required)")
	cmd.Flags().StringVar(&input.Description, "description", "", "description")
	cmd.Flags().StringVar(&input.FileName, "file-name", "", "stored file name override")
	cmd.Flags().StringVar(&input.PrivateKey, "private-key", "", "private key content")
	cmd.Flags().StringVar(&input.SourcePath, "source", "", "copy key material from an existing file")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func keyUpdateCmd() *cobra.Command {
	var input services.SSHKeyInput
	cmd := &cobra.Command{
		Use:   "update <name>",
		Short: "Update a managed SSH key (use --name to rename)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			client, err := newAPIClient(rootFlags.Home)
			if err != nil {
				return err
			}
			if err := loadKeyMaterial(&input); err != nil {
				return err
			}
			if err := client.request(http.MethodPut, "/api/keys/"+url.PathEscape(args[0]), input, nil); err != nil {
				return err
			}
			fmt.Printf("Key %q updated.\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&input.Name, "name", "", "rename the key")
	cmd.Flags().StringVar(&input.Description, "description", "", "description")
	cmd.Flags().StringVar(&input.FileName, "file-name", "", "stored file name override")
	cmd.Flags().StringVar(&input.PrivateKey, "private-key", "", "private key content")
	cmd.Flags().StringVar(&input.SourcePath, "source", "", "copy key material from an existing file")
	return cmd
}

func keyRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a managed SSH key",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			client, err := newAPIClient(rootFlags.Home)
			if err != nil {
				return err
			}
			if err := client.request(http.MethodDelete, "/api/keys/"+url.PathEscape(args[0]), nil, nil); err != nil {
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
