package cmd

import (
	"fmt"

	"github.com/rzbill/rune/pkg/api/client"
	"github.com/spf13/cobra"
)

var (
	statusNamespace  string
	statusClientAddr string
)

// statusCmd provides a concise status summary of services in a namespace
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status summary for services",
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
	statusCmd.Flags().StringVarP(&statusNamespace, "namespace", "n", "default", "Namespace to summarize")
	statusCmd.Flags().StringVar(&statusClientAddr, "api-server", "", "Address of the API server")
}

func runStatus(cmd *cobra.Command, args []string) error {
	api, err := newAPIClient(statusClientAddr, "")
	if err != nil {
		return fmt.Errorf("failed to connect to API server: %w", err)
	}
	defer api.Close()

	sc := client.NewServiceClient(api)
	resp, err := sc.ListServices(statusNamespace, "", "")
	if err != nil {
		return fmt.Errorf("failed to list services: %w", err)
	}

	if len(resp) == 0 {
		fmt.Printf("No services found in namespace %s\n", statusNamespace)
		return nil
	}

	fmt.Printf("Services in %s:\n", statusNamespace)
	fmt.Printf("%-24s %-12s %-8s\n", "NAME", "STATUS", "SCALE")
	for _, s := range resp {
		fmt.Printf("%-24s %-12s %-8d\n", s.Name, string(s.Status), s.Scale)
	}
	return nil
}
