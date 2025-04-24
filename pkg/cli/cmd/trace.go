package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	follow        bool
	tail          int
	showHealth    bool
	showResources bool
)

// traceCmd represents the trace command
var traceCmd = &cobra.Command{
	Use:   "trace [service name]",
	Short: "Show logs and diagnostics for a service",
	Long: `Show logs, health status, and diagnostics for a service.
For example:
  rune trace my-service
  rune trace my-service --follow
  rune trace my-service --health
  rune trace my-service --resources`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		serviceName := args[0]
		fmt.Printf("Showing information for service: %s\n", serviceName)
		fmt.Printf("Namespace: %s\n", namespace)

		if showHealth {
			fmt.Println("Health information:")
			fmt.Println("  Status: Healthy")
			fmt.Println("  Instances: 3/3 ready")
		}

		if showResources {
			fmt.Println("Resource usage:")
			fmt.Println("  CPU: 250m/500m")
			fmt.Println("  Memory: 128Mi/256Mi")
		}

		// Show logs (this would pull real logs in the actual implementation)
		fmt.Println("Logs:")
		fmt.Println("  2023-06-01 12:00:01 INFO  Service started")
		fmt.Println("  2023-06-01 12:00:02 INFO  Listening on port 8080")

		if follow {
			fmt.Println("Following logs (would continue in real implementation)...")
		}
	},
}

func init() {
	rootCmd.AddCommand(traceCmd)

	// Local flags for the trace command
	traceCmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow logs in real-time")
	traceCmd.Flags().IntVar(&tail, "tail", 10, "Number of lines to show from the end of logs")
	traceCmd.Flags().BoolVar(&showHealth, "health", false, "Show health status")
	traceCmd.Flags().BoolVar(&showResources, "resources", false, "Show resource usage")
	traceCmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "Namespace of the service")
}
