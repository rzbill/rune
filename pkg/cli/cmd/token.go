package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rzbill/rune/pkg/api/generated"
	"github.com/spf13/cobra"
)

func newTokenCmd() *cobra.Command {
	tokenCmd := &cobra.Command{Use: "token", Short: "Manage tokens (admin)"}
	tokenCmd.AddCommand(newTokenCreateCmd())
	tokenCmd.AddCommand(newTokenRevokeCmd())
	return tokenCmd
}

func newTokenCreateCmd() *cobra.Command {
	var desc, role, outFile string
	var ttl time.Duration
	var subjectID, subjectType, namespace, name string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new token and write it to a file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if outFile == "" {
				return fmt.Errorf("--out-file is required")
			}
			// Build client
			api, err := newAPIClient("", "")
			if err != nil {
				return err
			}
			defer api.Close()

			ac := generated.NewAuthServiceClient(api.Conn())
			req := &generated.CreateTokenRequest{
				Namespace:   namespace,
				Name:        name,
				SubjectId:   subjectID,
				SubjectType: subjectType,
				Roles:       []string{role},
				TtlSeconds:  int64(ttl / time.Second),
				Description: desc,
			}
			resp, err := ac.CreateToken(context.Background(), req)
			if err != nil {
				return err
			}
			if resp.Secret == "" {
				return fmt.Errorf("server did not return a token secret")
			}
			if err := os.WriteFile(outFile, []byte(resp.Secret), 0o600); err != nil {
				return fmt.Errorf("failed to write token to %s: %w", outFile, err)
			}
			fmt.Printf("Token created: id=%s name=%s namespace=%s written to %s\n", resp.Id, resp.Name, resp.Namespace, outFile)
			return nil
		},
	}
	cmd.Flags().StringVar(&desc, "description", "", "Description")
	cmd.Flags().StringVar(&role, "role", "readwrite", "Role binding for token (admin|readwrite|readonly)")
	cmd.Flags().DurationVar(&ttl, "ttl", 0, "Time-to-live (e.g., 1h). 0 means no expiry")
	cmd.Flags().StringVar(&outFile, "out-file", "", "Output file for token (0600)")
	cmd.Flags().StringVar(&subjectID, "subject-id", "", "Subject ID (user/service account id)")
	cmd.Flags().StringVar(&subjectType, "subject-type", "user", "Subject type (user|service)")
	cmd.Flags().StringVar(&namespace, "namespace", "system", "Namespace to store token in")
	cmd.Flags().StringVar(&name, "name", "", "Token name")
	return cmd
}

func newTokenRevokeCmd() *cobra.Command {
	var namespace, name string
	cmd := &cobra.Command{
		Use:   "revoke",
		Short: "Revoke a token",
		RunE: func(cmd *cobra.Command, args []string) error {
			api, err := newAPIClient("", "")
			if err != nil {
				return err
			}
			defer api.Close()

			ac := generated.NewAuthServiceClient(api.Conn())
			resp, err := ac.RevokeToken(context.Background(), &generated.RevokeTokenRequest{Namespace: namespace, Name: name})
			if err != nil {
				return err
			}
			if !resp.Revoked {
				return fmt.Errorf("token not revoked")
			}
			fmt.Println("Token revoked")
			return nil
		},
	}
	cmd.Flags().StringVar(&namespace, "namespace", "system", "Namespace")
	cmd.Flags().StringVar(&name, "name", "", "Token name")
	return cmd
}
