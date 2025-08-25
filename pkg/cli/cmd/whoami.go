package cmd

import (
	"context"
	"fmt"

	"github.com/rzbill/rune/pkg/api/generated"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newWhoAmICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Show current identity and context",
		RunE: func(cmd *cobra.Command, args []string) error {
			server := viper.GetString("contexts.default.server")
			token := viper.GetString("contexts.default.token")
			ns := viper.GetString("contexts.default.defaultNamespace")
			if ns == "" {
				ns = "default"
			}

			api, err := newAPIClient("", "")
			if err != nil {
				// Fallback to local display only
				fmt.Printf("Server: %s\nDefault Namespace: %s\nToken: %s\n", server, ns, maskToken(token))
				return nil
			}
			defer api.Close()

			ac := generated.NewAuthServiceClient(api.Conn())
			resp, err := ac.WhoAmI(context.Background(), &generated.WhoAmIRequest{})
			if err != nil || resp == nil || resp.SubjectId == "" {
				// Fallback to local
				fmt.Printf("Server: %s\nDefault Namespace: %s\nToken: %s\n", server, ns, maskToken(token))
				return nil
			}

			// Display user information with enhanced details
			fmt.Printf("Server: %s\nDefault Namespace: %s\nSubject ID: %s\n", server, ns, resp.SubjectId)

			if resp.SubjectName != "" {
				fmt.Printf("Name: %s\n", resp.SubjectName)
			}

			if resp.SubjectEmail != "" {
				fmt.Printf("Email: %s\n", resp.SubjectEmail)
			}

			if len(resp.Policies) > 0 {
				fmt.Printf("Policies: %v\n", resp.Policies)
			}

			fmt.Printf("Token: %s\n", maskToken(token))
			return nil
		},
	}
	return cmd
}

func maskToken(t string) string {
	if len(t) <= 6 {
		return "******"
	}
	return t[:3] + "******" + t[len(t)-3:]
}
