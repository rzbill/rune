package cmd

import (
	"context"
	"fmt"
	"sort"

	"github.com/rzbill/rune/pkg/api/client"
	"github.com/rzbill/rune/pkg/api/generated"
	resourceWatcher "github.com/rzbill/rune/pkg/cli/watcher"
	"github.com/rzbill/rune/pkg/types"
	"github.com/spf13/cobra"
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
	getClientAddr   string
	watchTimeout    string
	getServiceName  string
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
	"svc":        "service",
	"inst":       "instance",
	"ns":         "namespace",
	"cfg":        "config",
	"configmap":  "configmap",
	"configmaps": "configmap",
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
	getCmd.Flags().StringVar(&getServiceName, "service-name", "", "Filter instances by service name")

	// Remove api-key flag; token comes from config/env
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
	apiClient, err := newAPIClient(getClientAddr, "")
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
	case "ns":
		return handleNamespaceGet(ctx, cmd, apiClient, resourceName)
	case "secret":
		return handleSecretGet(ctx, cmd, apiClient, resourceName)
	case "configmap":
		return handleConfigMapGet(ctx, cmd, apiClient, resourceName)
	default:
		return fmt.Errorf("unsupported resource type: %s", args[0])
	}
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
	instanceClient := client.NewInstanceClient(apiClient)

	// If watch mode is enabled, handle watching
	if watchResources {
		return watchInstances(ctx, instanceClient, resourceName)
	}

	// If a specific instance name is provided, get that instance
	if resourceName != "" {
		namespace := getNamespace
		instance, err := instanceClient.GetInstance(namespace, resourceName)
		if err != nil {
			return fmt.Errorf("failed to get instance: %w", err)
		}

		// Render a single instance
		if getOutputFormat == "" {
			// Create table for a single instance
			table := NewResourceTable()
			table.RenderInstances([]*types.Instance{instance})
			return nil
		}

		// Handle JSON/YAML output for a single instance
		return outputResource([]*types.Instance{instance}, cmd)
	}

	// Get all instances with optional filtering
	instances, err := instanceClient.ListInstances(getNamespace, getServiceName, labelSelector, fieldSelector)
	if err != nil {
		return fmt.Errorf("failed to list instances: %w", err)
	}

	// Render the instances
	if getOutputFormat == "" {
		// Create table with configured options
		table := NewResourceTable()
		table.AllNamespaces = allNamespaces
		table.ShowLabels = showLabels
		table.RenderInstances(instances)
		return nil
	}

	// Handle JSON/YAML output
	return outputResource(instances, cmd)
}

// handleNamespaceGet handles get operations for namespaces
func handleNamespaceGet(ctx context.Context, cmd *cobra.Command, apiClient *client.Client, resourceName string) error {
	namespaceClient := client.NewNamespaceClient(apiClient)

	// If watch mode is enabled, handle watching
	if watchResources {
		return watchNamespaces(ctx, namespaceClient, resourceName)
	}

	// If a specific namespace name is provided, get that namespace
	if resourceName != "" {
		namespace, err := namespaceClient.GetNamespace(resourceName)
		if err != nil {
			return fmt.Errorf("failed to get namespace %s: %w", resourceName, err)
		}

		return outputResource([]*types.Namespace{namespace}, cmd)
	}

	// Otherwise, list all namespaces
	namespaces, err := namespaceClient.ListNamespaces(nil, nil)
	if err != nil {
		return fmt.Errorf("failed to list namespaces: %w", err)
	}

	// Sort namespaces
	sortNamespaces(namespaces, sortBy)

	// Apply limit if specified
	if limit > 0 && len(namespaces) > limit {
		namespaces = namespaces[:limit]
	}

	return outputResource(namespaces, cmd)
}

// handleSecretGet handles get operations for secrets
func handleSecretGet(ctx context.Context, cmd *cobra.Command, apiClient *client.Client, resourceName string) error {
	secretClient := client.NewSecretClient(apiClient)

	// If a specific secret name is provided, get that secret
	if resourceName != "" {
		namespace := getNamespace
		secret, err := secretClient.GetSecret(namespace, resourceName)
		if err != nil {
			return fmt.Errorf("failed to get secret %s: %w", resourceName, err)
		}

		return outputResource([]*types.Secret{secret}, cmd)
	}

	// Otherwise, list secrets based on the namespace flag
	var secrets []*types.Secret
	var err error

	if allNamespaces {
		// List secrets across all namespaces using the asterisk wildcard
		secrets, err = secretClient.ListSecrets("*", labelSelector, fieldSelector)
		if err != nil {
			return fmt.Errorf("failed to list secrets across all namespaces: %w", err)
		}
	} else {
		secrets, err = secretClient.ListSecrets(getNamespace, labelSelector, fieldSelector)
		if err != nil {
			return fmt.Errorf("failed to list secrets: %w", err)
		}
	}

	// Apply limit if specified
	if limit > 0 && len(secrets) > limit {
		secrets = secrets[:limit]
	}

	return outputResource(secrets, cmd)
}

// handleConfigMapGet handles get operations for configs
func handleConfigMapGet(ctx context.Context, cmd *cobra.Command, apiClient *client.Client, resourceName string) error {
	configMapClient := client.NewConfigMapClient(apiClient)

	// If a specific config name is provided, get that config
	if resourceName != "" {
		namespace := getNamespace
		config, err := configMapClient.GetConfigMap(namespace, resourceName)
		if err != nil {
			return fmt.Errorf("failed to get config %s: %w", resourceName, err)
		}

		return outputResource([]*types.ConfigMap{config}, cmd)
	}

	// Otherwise, list configmaps based on the namespace flag
	var configmaps []*types.ConfigMap
	var err error

	if allNamespaces {
		// List configs across all namespaces using the asterisk wildcard
		configmaps, err = configMapClient.ListConfigMaps("*", labelSelector, fieldSelector)
		if err != nil {
			return fmt.Errorf("failed to list configs across all namespaces: %w", err)
		}
	} else {
		configmaps, err = configMapClient.ListConfigMaps(getNamespace, labelSelector, fieldSelector)
		if err != nil {
			return fmt.Errorf("failed to list configs: %w", err)
		}
	}

	// Apply limit if specified
	if limit > 0 && len(configmaps) > limit {
		configmaps = configmaps[:limit]
	}

	return outputResource(configmaps, cmd)
}

// watchServices watches services for changes
func watchServices(ctx context.Context, serviceClient resourceWatcher.ServiceWatcher, resourceName string) error {
	// Create and configure the resource watcher
	watcher := resourceWatcher.NewResourceWatcher()
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
	watcher.SetResourceToRowsRenderer(resourceWatcher.DefaultServiceResourceToRows)
	watcher.SetEventRenderer(resourceWatcher.DefaultServiceEventRenderer)

	// Create adapter to convert ServiceWatcher to ResourceToWatch
	adapter := &resourceWatcher.ServiceWatcherAdapter{ServiceWatcher: serviceClient}

	// Start watching
	return watcher.Watch(ctx, adapter)
}

// watchNamespaces watches namespaces for changes
func watchNamespaces(ctx context.Context, namespaceClient *client.NamespaceClient, resourceName string) error {
	// For now, implement basic watching using the client
	watchCh, err := namespaceClient.WatchNamespaces(nil, nil)
	if err != nil {
		return fmt.Errorf("failed to watch namespaces: %w", err)
	}

	// Print initial namespaces
	namespaces, err := namespaceClient.ListNamespaces(nil, nil)
	if err != nil {
		return fmt.Errorf("failed to list initial namespaces: %w", err)
	}

	// Create table for display
	table := NewResourceTable()
	table.ShowHeaders = !noHeaders
	table.ShowLabels = showLabels

	// Display initial state
	if len(namespaces) > 0 {
		table.RenderNamespaces(namespaces)
	}

	fmt.Println("Watching namespaces... (press Ctrl+C to stop)")

	// Watch for changes
	for {
		select {
		case <-ctx.Done():
			return nil
		case _, ok := <-watchCh:
			if !ok {
				return nil
			}
			// For now, just re-list and display
			namespaces, err := namespaceClient.ListNamespaces(nil, nil)
			if err != nil {
				fmt.Printf("Error listing namespaces: %v\n", err)
				continue
			}
			// Clear screen and re-render (simple approach)
			fmt.Print("\033[2J\033[H") // Clear screen
			table.RenderNamespaces(namespaces)
		}
	}
}

// watchInstances watches instances for changes
func watchInstances(ctx context.Context, instanceClient resourceWatcher.InstanceWatcher, resourceName string) error {
	// Create and configure the resource watcher
	watcher := resourceWatcher.NewResourceWatcher()
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

	// Use default renderers for instances
	watcher.SetResourceToRowsRenderer(resourceWatcher.DefaultInstanceResourceToRows)
	watcher.SetEventRenderer(resourceWatcher.DefaultInstanceEventRenderer)

	// Create adapter to convert InstanceWatcher to ResourceToWatch
	adapter := &resourceWatcher.InstanceWatcherAdapter{InstanceWatcher: instanceClient}

	// Start watching
	return watcher.Watch(ctx, adapter)
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

// outputDeleteTable outputs deletion operations in a formatted table
func outputDeleteTable(operations []*generated.DeletionOperation) error {
	// Create and configure the table renderer
	table := NewResourceTable()
	table.ShowHeaders = !noHeaders
	table.ShowLabels = showLabels

	// Render the table
	return table.RenderDeletionOperations(operations)
}

// outputSecretsTable outputs secrets in a formatted table
func outputSecretsTable(secrets []*types.Secret) error {
	// Create and configure the table renderer
	table := NewResourceTable()
	table.ShowHeaders = !noHeaders
	table.ShowLabels = showLabels

	// Render the table
	return table.RenderSecrets(secrets)
}

// outputConfigmapsTable outputs configmaps in a formatted table
func outputConfigmapsTable(configmaps []*types.ConfigMap) error {
	// Create and configure the table renderer
	table := NewResourceTable()
	table.ShowHeaders = !noHeaders
	table.ShowLabels = showLabels

	// Render the table
	return table.RenderConfigmaps(configmaps)
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
			return services[i].Metadata.CreatedAt.Before(services[j].Metadata.CreatedAt)
		})
	case "status":
		sort.Slice(services, func(i, j int) bool {
			return string(services[i].Status) < string(services[j].Status)
		})
	}
}

// sortNamespaces sorts namespaces based on the specified sort field
func sortNamespaces(namespaces []*types.Namespace, sortField string) {
	switch sortField {
	case "name":
		sort.Slice(namespaces, func(i, j int) bool {
			return namespaces[i].Name < namespaces[j].Name
		})
	case "creationTime", "age":
		sort.Slice(namespaces, func(i, j int) bool {
			return namespaces[i].CreatedAt.Before(namespaces[j].CreatedAt)
		})
	}
}
