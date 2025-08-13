package cmd

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/rzbill/rune/pkg/api/client"
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
			ns := viper.GetString("contexts.default.namespace")

			// Call server WhoAmI when possible
			opts := client.DefaultClientOptions()
			if server != "" {
				// Map HTTP server URL to gRPC address host:port.
				host := server
				if u, err := url.Parse(server); err == nil && u.Host != "" {
					host = u.Host
				}
				// Default common mapping: 8081 (gateway) -> 8443 (gRPC)
				if strings.HasSuffix(host, ":8081") {
					host = strings.TrimSuffix(host, ":8081") + ":8443"
				}
				if !strings.Contains(host, ":") {
					host = host + ":8443"
				}
				opts.Address = host
			}
			if token != "" {
				opts.Token = token
			} else if t, ok := getEnv("RUNE_TOKEN"); ok {
				opts.Token = t
			}

			api, err := client.NewClient(opts)
			if err != nil {
				// Fallback to local display only
				fmt.Printf("Server: %s\nNamespace: %s\nToken: %s\n", server, ns, maskToken(token))
				return nil
			}
			defer api.Close()

			ac := generated.NewAuthServiceClient(api.Conn())
			resp, err := ac.WhoAmI(context.Background(), &generated.WhoAmIRequest{})
			if err != nil || resp == nil || resp.SubjectId == "" {
				// Fallback to local
				fmt.Printf("Server: %s\nNamespace: %s\nToken: %s\n", server, ns, maskToken(token))
				return nil
			}

			fmt.Printf("Server: %s\nNamespace: %s\nSubject: %s\nRoles: %v\nToken: %s\n", server, ns, resp.SubjectId, resp.Roles, maskToken(token))
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
