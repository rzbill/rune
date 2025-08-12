package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/rzbill/rune/pkg/api/client"
	"github.com/rzbill/rune/pkg/api/generated"
	"github.com/rzbill/rune/pkg/cli/format"
	"github.com/spf13/cobra"
)

var (
	startNamespace  string
	startReplicas   int
	startWait       bool
	startNoWait     bool
	startTimeout    time.Duration
	startClientKey  string
	startClientAddr string
)

// startCmd represents the start command (service)
var startCmd = &cobra.Command{
	Use:   "start <service-name>",
	Short: "Start a service by scaling it up (default 1)",
	Args:  cobra.ExactArgs(1),
	RunE:  runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)

	startCmd.Flags().StringVarP(&startNamespace, "namespace", "n", "default", "Namespace of the service")
	startCmd.Flags().IntVar(&startReplicas, "replicas", 1, "Number of instances to scale to")
	startCmd.Flags().BoolVar(&startWait, "wait", true, "Wait for service to reach desired replicas")
	startCmd.Flags().BoolVar(&startNoWait, "no-wait", false, "Don't wait for service to reach desired replicas")
	startCmd.Flags().DurationVar(&startTimeout, "timeout", 5*time.Minute, "Timeout for wait operation")

	// API client flags
	startCmd.Flags().StringVar(&startClientKey, "api-key", "", "API key for authentication")
	startCmd.Flags().StringVar(&startClientAddr, "api-server", "", "Address of the API server")
}

func runStart(cmd *cobra.Command, args []string) error {
	serviceName := args[0]

	if startReplicas < 1 {
		return fmt.Errorf("--replicas must be >= 1")
	}

	// Create API client
	options := client.DefaultClientOptions()
	if startClientAddr != "" {
		options.Address = startClientAddr
	}
	if startClientKey != "" {
		options.APIKey = startClientKey
	}
	apiClient, err := client.NewClient(options)
	if err != nil {
		return fmt.Errorf("failed to connect to API server: %w", err)
	}
	defer apiClient.Close()

	svcClient := client.NewServiceClient(apiClient)

	// Validate service exists
	svc, err := svcClient.GetService(startNamespace, serviceName)
	if err != nil {
		return fmt.Errorf("failed to get service %s/%s: %w", startNamespace, serviceName, err)
	}

	fmt.Printf("Starting service %s (current scale: %d) → %d\n", format.Highlight(serviceName), svc.Scale, startReplicas)

	req := &generated.ScaleServiceRequest{
		Name:      serviceName,
		Namespace: startNamespace,
		Scale:     int32(startReplicas),
		Mode:      generated.ScalingMode_SCALING_MODE_IMMEDIATE,
	}

	if _, err := svcClient.ScaleServiceWithRequest(req); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	if startWait && !startNoWait {
		ctx, cancel := context.WithTimeout(context.Background(), startTimeout)
		defer cancel()
		if err := waitForScalingComplete(apiClient, ctx, serviceName, startNamespace, startReplicas); err != nil {
			return err
		}
		fmt.Printf("%s Service %s started with %d instances\n", format.Success("✓"), format.Highlight(serviceName), startReplicas)
	}

	return nil
}
