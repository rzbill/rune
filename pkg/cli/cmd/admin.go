package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/rzbill/rune/pkg/api/generated"
	"github.com/spf13/cobra"
)

func newAdminCmd() *cobra.Command {
	admin := &cobra.Command{Use: "admin", Short: "Administrative operations (bootstrap, token, user, policy)"}
	admin.AddCommand(newAdminBootstrapCmd())
	admin.AddCommand(newAdminTokenCmd())
	admin.AddCommand(newAdminUserCmd())
	admin.AddCommand(newAdminPolicyCmd())
	return admin
}

func newAdminBootstrapCmd() *cobra.Command {
	var outFile string
	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Bootstrap root management token (server-enforced local-only if configured)",
		RunE: func(cmd *cobra.Command, args []string) error {
			api, err := newAPIClient("", "")
			if err != nil {
				return err
			}
			defer api.Close()
			ac := generated.NewAdminServiceClient(api.Conn())
			resp, err := ac.AdminBootstrap(context.Background(), &generated.AdminBootstrapRequest{})
			if err != nil {
				return err
			}
			if resp.TokenSecret == "" {
				return fmt.Errorf("server did not return a token secret")
			}
			if outFile != "" {
				if err := os.WriteFile(outFile, []byte(resp.TokenSecret), 0o600); err != nil {
					return fmt.Errorf("failed to write token: %w", err)
				}
				fmt.Println("Bootstrap token written to", outFile)
				return nil
			}
			fmt.Println(resp.TokenSecret)
			return nil
		},
	}
	cmd.Flags().StringVar(&outFile, "out-file", "", "Write token to file (0600). If empty, print to stdout")
	return cmd
}

func newAdminTokenCmd() *cobra.Command {
	c := &cobra.Command{Use: "token", Short: "Manage tokens (admin)"}
	c.AddCommand(newAdminTokenCreateCmd())
	c.AddCommand(newAdminTokenRevokeCmd())
	c.AddCommand(newAdminTokenListCmd())
	return c
}

func newAdminUserCmd() *cobra.Command {
	c := &cobra.Command{Use: "user", Short: "Manage users (subjects)"}
	c.AddCommand(newAdminUserCreateCmd())
	c.AddCommand(newAdminUserListCmd())
	return c
}

func newAdminPolicyCmd() *cobra.Command {
	c := &cobra.Command{Use: "policy", Short: "Manage policies"}
	c.AddCommand(newAdminPolicyCreateCmd())
	c.AddCommand(newAdminPolicyListCmd())
	c.AddCommand(newAdminPolicyGetCmd())
	c.AddCommand(newAdminPolicyDeleteCmd())
	c.AddCommand(newAdminPolicyAttachCmd())
	c.AddCommand(newAdminPolicyDetachCmd())
	return c
}
