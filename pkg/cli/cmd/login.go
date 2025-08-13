package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newLoginCmd() *cobra.Command {
	var server string
	var namespace string
	var token string
	var tokenFile string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login by writing ~/.rune/config.yaml (or --config path)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if token == "" && tokenFile == "" {
				return fmt.Errorf("must provide --token or --token-file")
			}
			if token == "" && tokenFile != "" {
				b, err := os.ReadFile(tokenFile)
				if err != nil {
					return err
				}
				token = string(b)
			}
			// Respect global --config flag if provided
			var cfgPath string
			if cfgFile != "" {
				if err := os.MkdirAll(filepath.Dir(cfgFile), 0700); err != nil {
					return err
				}
				cfgPath = cfgFile
			} else {
				home, err := os.UserHomeDir()
				if err != nil {
					return err
				}
				cfgDir := filepath.Join(home, ".rune")
				if err := os.MkdirAll(cfgDir, 0700); err != nil {
					return err
				}
				cfgPath = filepath.Join(cfgDir, "config.yaml")
			}
			nsLine := ""
			if namespace != "" {
				nsLine = fmt.Sprintf("\n    namespace: %s", namespace)
			}
			content := fmt.Sprintf("current-context: default\ncontexts:\n  default:\n    server: %s\n    token: %s%s\n", server, token, nsLine)
			if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
				// If permission denied in default location, fall back to XDG config path
				if cfgFile == "" && os.IsPermission(err) {
					xdg := os.Getenv("XDG_CONFIG_HOME")
					if xdg == "" {
						home, _ := os.UserHomeDir()
						xdg = filepath.Join(home, ".config")
					}
					altDir := filepath.Join(xdg, "rune")
					if mkErr := os.MkdirAll(altDir, 0700); mkErr == nil {
						alt := filepath.Join(altDir, "config.yaml")
						if wErr := os.WriteFile(alt, []byte(content), 0600); wErr == nil {
							fmt.Printf("Wrote config to %s (fallback). You can also pass --config to choose a path.\n", alt)
							return nil
						}
					}
				}
				return fmt.Errorf("failed to write config at %s: %w", cfgPath, err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&server, "server", "http://localhost:8081", "Rune API server URL")
	cmd.Flags().StringVar(&namespace, "namespace", "", "Optional default namespace")
	cmd.Flags().StringVar(&token, "token", "", "Bearer token value")
	cmd.Flags().StringVar(&tokenFile, "token-file", "", "Path to file containing the bearer token")
	return cmd
}
