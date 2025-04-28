package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/rzbill/rune/pkg/api/client"
	"github.com/rzbill/rune/pkg/types"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	// Get command flags
	getNamespace    string
	allNamespaces   bool
	getOutputFormat string
	watchResources  bool
	labelSelector   string
	fieldSelector   string
	sortBy          string
	showLabels      bool
	noHeaders       bool
	limit           int
	getClientAPIKey string
	getClientAddr   string
	watchTimeout    string
)

// Resource type abbreviations
var resourceAliases = map[string]string{
	// Full names
	"service":    "service",
	"services":   "service",
	"instance":   "instance",
	"instances":  "instance",
	"namespace":  "namespace",
	"namespaces": "namespace",
	"job":        "job",
	"jobs":       "job",
	"config":     "config",
	"configs":    "config",
	"secret":     "secret",
	"secrets":    "secret",
	// Abbreviations
	"svc":  "service",
	"inst": "instance",
	"ns":   "namespace",
	"cfg":  "config",
}

// ServiceWatcher defines the interface for watching services
type ServiceWatcher interface {
	WatchServices(namespace, labelSelector, fieldSelector string) (<-chan client.WatchEvent, error)
}

// getCmd represents the get command
var getCmd = &cobra.Command{
	Use:   "get <resource-type> [resource-name] [flags]",
	Short: "Display one or many resources",
	Long: `Display one or many resources.
The resource types include:
  * services (svc)
  * instances (inst)
  * namespaces (ns)
  * jobs
  * configs
  * secrets`,
	Args: cobra.MinimumNArgs(1),
	RunE: runGet,
	Example: `  # List all services in the current namespace
  rune get services
  
  # List a specific service in YAML format
  rune get service my-service --output=yaml
  
  # List all services in all namespaces
  rune get services --all-namespaces
  
  # List services with real-time updates
  rune get services --watch
  
  # List services filtered by labels
  rune get services --selector=app=backend`,
}

func init() {
	rootCmd.AddCommand(getCmd)

	// Local flags for the get command
	getCmd.Flags().StringVarP(&getNamespace, "namespace", "n", "default", "Namespace to list resources from")
	getCmd.Flags().BoolVarP(&allNamespaces, "all-namespaces", "A", false, "List resources across all namespaces")
	getCmd.Flags().StringVarP(&getOutputFormat, "output", "o", "table", "Output format (table, json, yaml)")
	getCmd.Flags().BoolVarP(&watchResources, "watch", "w", false, "Watch for changes and update in real-time")
	getCmd.Flags().StringVarP(&labelSelector, "selector", "l", "", "Filter resources by label selector (e.g., app=frontend)")
	getCmd.Flags().StringVar(&fieldSelector, "field-selector", "", "Filter resources by field selector (e.g., status=running)")
	getCmd.Flags().StringVar(&sortBy, "sort-by", "name", "Sort resources by field (name, creationTime, status)")
	getCmd.Flags().BoolVar(&showLabels, "show-labels", false, "Show labels as the last column (table output only)")
	getCmd.Flags().BoolVar(&noHeaders, "no-headers", false, "Don't print headers for table output")
	getCmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of resources to list (0 for unlimited)")
	getCmd.Flags().StringVar(&watchTimeout, "timeout", "", "Timeout for watch operations (e.g., 30s, 5m, 1h) - default is no timeout")

	// API client flags
	getCmd.Flags().StringVar(&getClientAPIKey, "api-key", "", "API key for authentication")
	getCmd.Flags().StringVar(&getClientAddr, "api-server", "", "Address of the API server")
}

// runGet is the main entry point for the get command
func runGet(cmd *cobra.Command, args []string) error {
	// Resolve resource type from the first argument
	resourceType, err := resolveResourceType(args[0])
	if err != nil {
		return err
	}

	// Get resource name if provided
	var resourceName string
	if len(args) > 1 {
		resourceName = args[1]
	}

	// Create API client
	apiClient, err := createGetAPIClient()
	if err != nil {
		return fmt.Errorf("failed to connect to API server: %w", err)
	}
	defer apiClient.Close()

	// Create context
	ctx := context.Background()

	// Process the request based on resource type
	switch resourceType {
	case "service":
		return handleServiceGet(ctx, cmd, apiClient, resourceName)
	case "instance":
		return handleInstanceGet(ctx, cmd, apiClient, resourceName)
	case "namespace":
		return handleNamespaceGet(ctx, cmd, apiClient, resourceName)
	default:
		return fmt.Errorf("unsupported resource type: %s", args[0])
	}
}

// resolveResourceType resolves a resource type from user input to a canonical type
func resolveResourceType(input string) (string, error) {
	// Lookup the resource type alias
	if resourceType, ok := resourceAliases[input]; ok {
		return resourceType, nil
	}

	return "", fmt.Errorf("unsupported resource type: %s", input)
}

// handleServiceGet handles get operations for services
func handleServiceGet(ctx context.Context, cmd *cobra.Command, apiClient *client.Client, resourceName string) error {
	serviceClient := client.NewServiceClient(apiClient)

	// If watch mode is enabled, handle watching
	if watchResources {
		return watchServices(ctx, serviceClient, resourceName)
	}

	// If a specific service name is provided, get that service
	if resourceName != "" {
		namespace := getNamespace
		service, err := serviceClient.GetService(namespace, resourceName)
		if err != nil {
			return fmt.Errorf("failed to get service %s: %w", resourceName, err)
		}

		return outputResource([]*types.Service{service}, cmd)
	}

	// Otherwise, list services based on the namespace flag
	var services []*types.Service
	var err error

	if allNamespaces {
		// List services across all namespaces using the asterisk wildcard
		services, err = serviceClient.ListServices("*", labelSelector, fieldSelector)
		if err != nil {
			return fmt.Errorf("failed to list services across all namespaces: %w", err)
		}
	} else {
		services, err = serviceClient.ListServices(getNamespace, labelSelector, fieldSelector)
		if err != nil {
			return fmt.Errorf("failed to list services: %w", err)
		}
	}

	// Sort services (client-side sorting is still fine)
	sortServices(services, sortBy)

	// Apply limit if specified
	if limit > 0 && len(services) > limit {
		services = services[:limit]
	}

	return outputResource(services, cmd)
}

// handleInstanceGet handles get operations for instances
func handleInstanceGet(ctx context.Context, cmd *cobra.Command, apiClient *client.Client, resourceName string) error {
	// TODO: Implement instance listing when API client supports it
	return fmt.Errorf("instance listing not yet implemented")
}

// handleNamespaceGet handles get operations for namespaces
func handleNamespaceGet(ctx context.Context, cmd *cobra.Command, apiClient *client.Client, resourceName string) error {
	// TODO: Implement namespace listing when API client supports it
	return fmt.Errorf("namespace listing not yet implemented")
}

// watchServices watches services for changes
func watchServices(ctx context.Context, serviceClient ServiceWatcher, resourceName string) error {
	// Create and configure the resource watcher
	watcher := NewResourceWatcher()
	watcher.Namespace = getNamespace
	watcher.AllNamespaces = allNamespaces
	watcher.ResourceName = resourceName
	watcher.LabelSelector = labelSelector
	watcher.FieldSelector = fieldSelector
	watcher.ShowHeaders = !noHeaders

	// Set timeout if specified
	if watchTimeout != "" {
		if err := watcher.SetTimeout(watchTimeout); err != nil {
			return err
		}
	}

	// Use default renderers for services
	watcher.SetResourceToRowsRenderer(DefaultServiceResourceToRows)
	watcher.SetEventRenderer(DefaultServiceEventRenderer)

	// Create adapter to convert ServiceWatcher to ResourceToWatch
	adapter := &ServiceWatcherAdapter{ServiceWatcher: serviceClient}

	// Start watching
	return watcher.Watch(ctx, adapter)
}

// outputResource outputs a resource in the specified format
func outputResource(resources interface{}, cmd *cobra.Command) error {
	switch getOutputFormat {
	case "json":
		return outputJSON(resources)
	case "yaml":
		return outputYAML(resources)
	case "table", "":
		return outputTable(resources)
	default:
		return fmt.Errorf("unsupported output format: %s", getOutputFormat)
	}
}

// outputJSON outputs resources in JSON format
func outputJSON(resources interface{}) error {
	jsonData, err := json.MarshalIndent(resources, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal resources to JSON: %w", err)
	}
	fmt.Println(string(jsonData))
	return nil
}

// outputYAML outputs resources in YAML format
func outputYAML(resources interface{}) error {
	yamlData, err := yaml.Marshal(resources)
	if err != nil {
		return fmt.Errorf("failed to marshal resources to YAML: %w", err)
	}
	fmt.Println(string(yamlData))
	return nil
}

// outputTable outputs resources in table format
func outputTable(resources interface{}) error {
	switch r := resources.(type) {
	case []*types.Service:
		return outputServicesTable(r)
	case []*types.Instance:
		return outputInstancesTable(r)
	case []*types.Namespace:
		return outputNamespacesTable(r)
	// TODO: Add more resource types here
	default:
		return fmt.Errorf("unsupported resource type for table output")
	}
}

// outputServicesTable outputs services in a formatted table
func outputServicesTable(services []*types.Service) error {
	// Create and configure the table renderer
	table := NewResourceTable()
	table.AllNamespaces = allNamespaces
	table.ShowHeaders = !noHeaders
	table.ShowLabels = showLabels

	// Render the table
	return table.RenderServices(services)
}

// outputInstancesTable outputs instances in a formatted table
func outputInstancesTable(instances []*types.Instance) error {
	// Create and configure the table renderer
	table := NewResourceTable()
	table.AllNamespaces = allNamespaces
	table.ShowHeaders = !noHeaders
	table.ShowLabels = showLabels

	// Render the table
	return table.RenderInstances(instances)
}

// outputNamespacesTable outputs namespaces in a formatted table
func outputNamespacesTable(namespaces []*types.Namespace) error {
	// Create and configure the table renderer
	table := NewResourceTable()
	table.ShowHeaders = !noHeaders
	table.ShowLabels = showLabels

	// Render the table
	return table.RenderNamespaces(namespaces)
}

// parseSelector parses a selector string into a map of key-value pairs
func parseSelector(selector string) (map[string]string, error) {
	result := make(map[string]string)
	if selector == "" {
		return result, nil
	}

	pairs := strings.Split(selector, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid selector format, expected key=value: %s", pair)
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			return nil, fmt.Errorf("empty key in selector: %s", pair)
		}
		if value == "" {
			return nil, fmt.Errorf("empty value in selector: %s", pair)
		}
		if strings.Contains(value, "=") {
			return nil, fmt.Errorf("invalid value format, contains additional equals sign: %s", value)
		}
		result[key] = value
	}

	return result, nil
}

// sortServices sorts services based on the specified sort field
func sortServices(services []*types.Service, sortField string) {
	switch sortField {
	case "name":
		sort.Slice(services, func(i, j int) bool {
			return services[i].Name < services[j].Name
		})
	case "creationTime", "age":
		sort.Slice(services, func(i, j int) bool {
			return services[i].CreatedAt.Before(services[j].CreatedAt)
		})
	case "status":
		sort.Slice(services, func(i, j int) bool {
			return string(services[i].Status) < string(services[j].Status)
		})
	}
}

// createGetAPIClient creates an API client with the configured options.
func createGetAPIClient() (*client.Client, error) {
	options := client.DefaultClientOptions()

	// Override defaults with command-line flags if set
	if getClientAddr != "" {
		options.Address = getClientAddr
	}

	if getClientAPIKey != "" {
		options.APIKey = getClientAPIKey
	} else {
		// Try to get API key from environment
		if apiKey, ok := os.LookupEnv("RUNE_API_KEY"); ok {
			options.APIKey = apiKey
		}
	}

	// Create the client
	return client.NewClient(options)
}
