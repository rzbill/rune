package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	namespace string
	tag       string
)

// castCmd represents the cast command
var castCmd = &cobra.Command{
	Use:   "cast [service file]",
	Short: "Deploy a service",
	Long: `Deploy a service defined in a YAML file.
For example:
  rune cast my-service.yml
  rune cast my-service.yml --namespace=production
  rune cast my-service.yml --tag=stable`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		serviceFile := args[0]
		fmt.Printf("Deploying service from file: %s\n", serviceFile)
		fmt.Printf("Namespace: %s\n", namespace)
		if tag != "" {
			fmt.Printf("Deployment tag: %s\n", tag)
		}
		// Actual implementation will be added in future tickets
		fmt.Println("This is a placeholder for the actual cast implementation")
	},
}

func init() {
	rootCmd.AddCommand(castCmd)

	// Local flags for the cast command
	castCmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "Namespace to deploy the service in")
	castCmd.Flags().StringVar(&tag, "tag", "", "Tag for this deployment (e.g., 'stable', 'canary')")
}
