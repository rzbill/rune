package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rzbill/rune/pkg/api/client"
	"github.com/rzbill/rune/pkg/cli/format"
	"github.com/rzbill/rune/pkg/cli/util"
	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/types"
	"github.com/spf13/cobra"
)

var (
	// Cast command flags
	namespace       string
	tag             string
	dryRun          bool
	detach          bool
	timeoutStr      string
	recursiveDir    bool
	clientAPIKey    string
	clientAddr      string
	forceGeneration bool
)

// castCmd represents the cast command
var castCmd = &cobra.Command{
	Use:   "cast [service files or directories...]",
	Short: "Deploy a service",
	Long: `Deploy a service defined in a YAML file.
For example:
  rune cast my-service.yml
  rune cast my-service.yml --namespace=production
  rune cast my-service.yml --tag=stable
  rune cast my-directory/ --recursive
  rune cast services/*.yaml
  rune cast my-service.yml --force`,
	Args: cobra.MinimumNArgs(1),
	RunE: runCast,
}

func init() {
	rootCmd.AddCommand(castCmd)

	// Local flags for the cast command
	castCmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "Namespace to deploy the service in")
	castCmd.Flags().StringVar(&tag, "tag", "", "Tag for this deployment (e.g., 'stable', 'canary')")
	castCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate the service definition without deploying it")
	castCmd.Flags().BoolVar(&detach, "detach", false, "Detach from the deployment and return immediately")
	castCmd.Flags().StringVar(&timeoutStr, "timeout", "5m", "Timeout for the wait operation")
	castCmd.Flags().BoolVarP(&recursiveDir, "recursive", "r", false, "Recursively process directories")
	castCmd.Flags().BoolVar(&forceGeneration, "force", false, "Force generation increment even if no changes are detected")

	// API client flags
	castCmd.Flags().StringVar(&clientAPIKey, "api-key", "", "API key for authentication")
	castCmd.Flags().StringVar(&clientAddr, "api-server", "", "Address of the API server")
}

// runCast is the main entry point for the cast command
func runCast(cmd *cobra.Command, args []string) error {
	startTime := time.Now()

	// Parse timeout duration
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return fmt.Errorf("invalid timeout value: %w", err)
	}

	// Expand file paths (including directories and glob patterns)
	filePaths, err := util.ExpandFilePaths(args, recursiveDir)
	if err != nil {
		return err
	}

	if len(filePaths) == 0 {
		return fmt.Errorf("no service files found")
	}

	// Print initial cast banner
	printCastBanner(args, detach)

	// Load, categorize, and validate resources
	resourceInfo, err := processResourceFiles(filePaths, args)
	if err != nil {
		return err
	}

	// If dry-run is enabled, we're done here
	if dryRun {
		fmt.Println("✅ Validation successful!")
		fmt.Println("💬 Use without --dry-run to deploy.")
		return nil
	}

	// Create API client
	apiClient, err := createAPIClient()
	if err != nil {
		return fmt.Errorf("failed to connect to API server: %w", err)
	}
	defer apiClient.Close()

	// Print deployment preparation
	fmt.Println("🚀 Preparing deployment plan...")
	fmt.Println()

	// Deploy resources
	deploymentResults, err := deployResources(apiClient, resourceInfo, timeout)
	if err != nil {
		return err
	}

	// Print summary based on deployment mode
	if detach {
		printDetachedModeSummary(deploymentResults, startTime)
	} else {
		printWatchModeSummary(deploymentResults, startTime)
	}

	return nil
}

// ResourceInfo holds information about resources to be deployed
type ResourceInfo struct {
	FilesByType     map[string][]string
	ServicesByFile  map[string][]*types.Service
	TotalResources  int
	SourceArguments []string
}

// processResourceFiles loads, categorizes, and validates resources from files
func processResourceFiles(filePaths []string, sourceArgs []string) (*ResourceInfo, error) {
	info := &ResourceInfo{
		FilesByType:     make(map[string][]string),
		ServicesByFile:  make(map[string][]*types.Service),
		TotalResources:  0,
		SourceArguments: sourceArgs,
	}

	// Print detected resources header
	fmt.Println("🧩 Validating specifications...")

	// First pass: load and validate services
	for _, filePath := range filePaths {
		fileName := filepath.Base(filePath)

		// Format the filename with fixed width padding for display
		fileNameDisplay := fmt.Sprintf("- %-20s", fileName)

		// Print filename (without newline)
		fmt.Print(fileNameDisplay)

		// Calculate and print dots
		totalWidth := 35
		dotsNeeded := totalWidth - len(fileNameDisplay)
		if dotsNeeded < 3 {
			dotsNeeded = 3
		}
		fmt.Print(strings.Repeat(".", dotsNeeded))
		fmt.Print(" ") // Space before validation result

		// Parse the service file
		serviceFile, err := types.ParseServiceFile(filePath)
		if err != nil {
			fmt.Println("❌") // Show failure
			return nil, fmt.Errorf("failed to parse file %s: %w", filePath, err)
		}

		// Get all services from the file
		serviceSpecs := serviceFile.GetServices()
		if len(serviceSpecs) == 0 {
			fmt.Println("⚠️ No services") // Show warning
			continue
		}

		// Convert specs to services and validate them
		services := make([]*types.Service, 0, len(serviceSpecs))
		validationSuccessful := true

		for _, spec := range serviceSpecs {
			// Validate the service spec
			if err := spec.Validate(); err != nil {
				fmt.Println("❌") // Show failure

				// Try to get line number for better error reporting
				line, ok := serviceFile.GetLineInfo(spec.Name)
				if ok {
					return nil, fmt.Errorf("validation error in %s, line %d, service %s: %w",
						filePath, line, spec.Name, err)
				}
				return nil, fmt.Errorf("validation error in %s, service %s: %w",
					filePath, spec.Name, err)
			}

			// Convert to service
			service, err := spec.ToService()
			if err != nil {
				fmt.Println("❌") // Show failure
				validationSuccessful = false
				return nil, fmt.Errorf("failed to convert service spec: %w", err)
			}

			services = append(services, service)
		}

		// Show success checkmark if all validations passed
		if validationSuccessful {
			fmt.Println("✓")
		}

		info.ServicesByFile[filePath] = services
		info.TotalResources += len(services)

		// Categorize resources by type
		for _, service := range services {
			resourceType := guessResourceTypeFromName(filePath, service.Name)
			if _, exists := info.FilesByType[resourceType]; !exists {
				info.FilesByType[resourceType] = []string{}
			}
			if !stringSliceContains(info.FilesByType[resourceType], fileName) {
				info.FilesByType[resourceType] = append(info.FilesByType[resourceType], fileName)
			}
		}
	}

	fmt.Println()

	// Print detected resources
	printResourceInfo(info)

	return info, nil
}

// DeploymentResult holds results of a deployment
type DeploymentResult struct {
	SuccessfulResources map[string][]string
	FailedResources     map[string]string
}

// deployResources handles the actual deployment of services to the cluster
func deployResources(apiClient *client.Client, info *ResourceInfo, timeout time.Duration) (*DeploymentResult, error) {
	results := &DeploymentResult{
		SuccessfulResources: make(map[string][]string),
		FailedResources:     make(map[string]string),
	}

	fmt.Println("📡 Casting resources to Rune Cluster...")

	serviceClient := client.NewServiceClient(apiClient)

	resourceCount := info.TotalResources
	resourceIndex := 0

	for filePath, services := range info.ServicesByFile {
		for _, service := range services {
			resourceIndex++

			// Apply namespace if not specified in the service
			if service.Namespace == "" {
				service.Namespace = namespace
			}

			// Add deployment tag metadata if specified
			if tag != "" {
				if service.Env == nil {
					service.Env = make(map[string]string)
				}
				service.Env["RUNE_DEPLOYMENT_TAG"] = tag
			}

			// Determine resource type for display
			resourceType := guessResourceTypeFromName(filePath, service.Name)

			// Check if service already exists - determine if creating or updating
			action := "Creating"
			_, err := serviceClient.GetService(service.Namespace, service.Name)
			if err == nil {
				action = "Deploying"
			}

			fmt.Printf("  [%d/%d] %s %s \"%s\" ",
				resourceIndex, resourceCount,
				action,
				resourceType,
				format.Highlight(service.Name))

			// Print dots for alignment
			dotCount := 25 - len(action) - len(resourceType) - len(service.Name)
			if dotCount < 3 {
				dotCount = 3
			}
			fmt.Print(strings.Repeat(".", dotCount))

			var deployErr error
			if action == "Creating" {
				deployErr = serviceClient.CreateService(service)
			} else {
				deployErr = serviceClient.UpdateService(service, forceGeneration)
			}

			if deployErr != nil {
				fmt.Println(" ❌")
				results.FailedResources[service.Name] = deployErr.Error()

				// For error cases, show the error and stop deployment
				fmt.Println()
				fmt.Printf("❌ Failed to deploy %s \"%s\"\n", resourceType, service.Name)
				fmt.Printf("Error: %s\n", deployErr)

				// If we have successful resources, show them
				if len(results.SuccessfulResources) > 0 {
					fmt.Println()
					fmt.Println("✅ Successfully deployed:")
					for rType, names := range results.SuccessfulResources {
						for _, name := range names {
							fmt.Printf("- %s: %s\n", rType, name)
						}
					}
				}

				fmt.Println()
				fmt.Println("🛑 Aborted remaining deployments to maintain consistency.")
				fmt.Printf("Hint: Fix the %s spec and re-run 'rune cast %s'\n", strings.ToLower(resourceType), info.SourceArguments[0])

				return results, fmt.Errorf("deployment failed")
			}

			fmt.Println(" ✓")

			// Add to successful resources
			results.SuccessfulResources[resourceType] = append(results.SuccessfulResources[resourceType], service.Name)

			// If detach is not enabled, wait for the service to be ready
			if !detach {
				// For watch mode, we wait for each service to become ready
				waitErr := waitForServiceReady(serviceClient, service.Namespace, service.Name, timeout)
				if waitErr != nil {
					fmt.Printf("    ⚠️  Service is still starting (timeout: %s): %s\n",
						timeoutStr, waitErr)
				}
			}
		}
	}

	fmt.Println()
	return results, nil
}

// printCastBanner prints the initial banner for the cast command
func printCastBanner(args []string, isDetached bool) {
	if isDetached {
		fmt.Println("\n🔮 Rune Cast Initiated (Detached Mode)")
	} else {
		fmt.Println("\n🔮 Rune Cast Initiated")
	}

	// Print source info
	fmt.Println("\n- Source:", format.Highlight(strings.Join(args, ", ")))
	fmt.Println()
}

// printResourceInfo displays information about detected resources
func printResourceInfo(info *ResourceInfo) {
	// Print detected resources
	fmt.Printf("- Detected %d resources:\n", info.TotalResources)
	for resourceType, files := range info.FilesByType {
		for _, file := range files {
			fmt.Printf("  - %s: %s\n", resourceType, file)
		}
	}

	fmt.Println("- Namespace:", format.Highlight(namespace))

	// Determine target display name
	targetDisplay := "local agent"
	if clientAddr != "" {
		targetDisplay = clientAddr
	} else {
		// Get default address from client options
		options := client.DefaultClientOptions()
		targetDisplay = options.Address
	}
	fmt.Printf("- Target: %s (%s)\n", format.Highlight(targetDisplay), targetDisplay)
	fmt.Println()

	if dryRun {
		fmt.Println("🧪 Running in dry-run mode (validation only)")
	}
}

// printDetachedModeSummary prints a summary for detached mode
func printDetachedModeSummary(results *DeploymentResult, startTime time.Time) {
	fmt.Println("🛫 Deployment initiated (no watch mode).")
	fmt.Println()
	fmt.Println("💬 Status commands:")

	// Suggest some commands for checking status
	for resourceType, names := range results.SuccessfulResources {
		if resourceType == "Service" || resourceType == "Function" {
			for _, name := range names {
				fmt.Printf("- rune trace %s\n", name)
			}
		}
	}
	fmt.Println("- rune status")
	fmt.Println("- rune list")

	elapsedTime := time.Since(startTime).Seconds()
	fmt.Printf("\n🏁 Done in %.1fs (deployment is ongoing in background).\n", elapsedTime)
}

// printWatchModeSummary prints a summary for watch mode
func printWatchModeSummary(results *DeploymentResult, startTime time.Time) {
	fmt.Println("✅ Successfully deployed:")

	// Show resources with endpoints where applicable
	for resourceType, names := range results.SuccessfulResources {
		for _, name := range names {
			endpoint := ""
			if resourceType == "Service" {
				endpoint = fmt.Sprintf(" (endpoint: http://%s.%s.rune)", name, namespace)
			} else if resourceType == "Function" {
				endpoint = fmt.Sprintf(" (endpoint: http://%s.%s.rune)", name, namespace)
			}
			fmt.Printf("- %s: %s%s\n", resourceType, name, endpoint)
		}
	}

	// Show tips
	fmt.Println()
	fmt.Println("🪄 Tips:")

	// Add service-specific tips
	for resourceType, names := range results.SuccessfulResources {
		if resourceType == "Service" {
			for _, name := range names {
				fmt.Printf("- Monitor: rune trace %s\n", name)
				fmt.Printf("- Scale Service: rune scale %s --replicas=2\n", name)
			}
		} else if resourceType == "Function" {
			for _, name := range names {
				fmt.Printf("- Function Logs: rune trace %s\n", name)
			}
		}
	}

	fmt.Println("- List Resources: rune list")

	elapsedTime := time.Since(startTime).Seconds()
	fmt.Printf("\n🎉 All resources ready in %.1fs\n", elapsedTime)
}

// determineResourceType guesses the resource type based on file path and name
func guessResourceTypeFromName(filePath, serviceName string) string {
	resourceType := "Service"
	if strings.Contains(filePath, "secret") || strings.Contains(serviceName, "secret") {
		resourceType = "Secret"
	} else if strings.Contains(filePath, "function") || strings.Contains(serviceName, "fn") {
		resourceType = "Function"
	}
	return resourceType
}

// contains checks if a string slice contains a specific value
func stringSliceContains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}

// createAPIClient creates an API client with the configured options.
func createAPIClient() (*client.Client, error) {
	options := client.DefaultClientOptions()

	// Override defaults with command-line flags if set
	if clientAddr != "" {
		options.Address = clientAddr
	}

	if clientAPIKey != "" {
		options.APIKey = clientAPIKey
	} else {
		// Try to get API key from environment
		if apiKey, ok := os.LookupEnv("RUNE_API_KEY"); ok {
			options.APIKey = apiKey
		}
	}

	// Create the client
	return client.NewClient(options)
}

// waitForServiceReady waits for a service to be fully deployed.
func waitForServiceReady(serviceClient *client.ServiceClient, namespace, name string, timeout time.Duration) error {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Poll interval
	pollInterval := 1 * time.Second

	// Create a ticker for polling
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Timeout
			return fmt.Errorf("timeout waiting for service %s to be ready", name)
		case <-ticker.C:
			// Poll the service status
			service, err := serviceClient.GetService(namespace, name)

			fmt.Println("service", service.Status)
			if err != nil {
				log.Debug("Error getting service status", log.Err(err))
				continue
			}

			// Check if the service is running
			if service.Status == types.ServiceStatusRunning {
				// Check that all instances are ready
				allReady := true
				readyCount := 0

				for _, instance := range service.Instances {
					if instance.Status == "Running" || instance.Status == "Ready" {
						readyCount++
					} else {
						allReady = false
					}
				}

				// If all instances are ready, or at least some are ready and the service scale is met
				if (allReady && len(service.Instances) > 0) || (readyCount >= service.Scale) {
					return nil
				}
			}
		}
	}
}
