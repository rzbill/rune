package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/rzbill/rune/pkg/api/client"
	"github.com/rzbill/rune/pkg/api/generated"
	"github.com/rzbill/rune/pkg/cli/format"
	"github.com/rzbill/rune/pkg/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
	"configmap":  "configmap",
	"configmaps": "configmap",
}

// ServiceWatcher defines the interface for watching services
type ServiceWatcher interface {
	WatchServices(namespace, labelSelector, fieldSelector string) (<-chan client.WatchEvent, context.CancelFunc, error)
}

// InstanceWatcher defines the interface for watching instances
type InstanceWatcher interface {
	WatchInstances(namespace, serviceId, labelSelector, fieldSelector string) (<-chan client.InstanceWatchEvent, error)
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
	case "secret":
		return handleSecretGet(ctx, cmd, apiClient, resourceName)
	case "configmap":
		return handleConfigMapGet(ctx, cmd, apiClient, resourceName)
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
	// TODO: Implement namespace handling
	return fmt.Errorf("namespace operations not yet implemented")
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

// watchInstances watches instances for changes
func watchInstances(ctx context.Context, instanceClient InstanceWatcher, resourceName string) error {
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

	// Use default renderers for instances
	watcher.SetResourceToRowsRenderer(DefaultInstanceResourceToRows)
	watcher.SetEventRenderer(DefaultInstanceEventRenderer)

	// Create adapter to convert InstanceWatcher to ResourceToWatch
	adapter := &InstanceWatcherAdapter{InstanceWatcher: instanceClient}

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
			return services[i].Metadata.CreatedAt.Before(services[j].Metadata.CreatedAt)
		})
	case "status":
		sort.Slice(services, func(i, j int) bool {
			return string(services[i].Status) < string(services[j].Status)
		})
	}
}

// sortInstances sorts a list of instances based on the sort criteria
func sortInstances(instances []*types.Instance, sortBy string) {
	switch sortBy {
	case "name":
		sort.Slice(instances, func(i, j int) bool {
			return instances[i].Name < instances[j].Name
		})
	case "age":
		sort.Slice(instances, func(i, j int) bool {
			return instances[i].CreatedAt.Before(instances[j].CreatedAt)
		})
	case "status":
		sort.Slice(instances, func(i, j int) bool {
			return string(instances[i].Status) < string(instances[j].Status)
		})
	case "service":
		sort.Slice(instances, func(i, j int) bool {
			return instances[i].ServiceID < instances[j].ServiceID
		})
	case "node":
		sort.Slice(instances, func(i, j int) bool {
			return instances[i].NodeID < instances[j].NodeID
		})
	default:
		// Default to sorting by name
		sort.Slice(instances, func(i, j int) bool {
			return instances[i].Name < instances[j].Name
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

	// Prefer token from config/env, fall back to deprecated API key
	// Read CLI config via viper if available
	if t := viper.GetString("contexts.default.token"); t != "" {
		options.Token = t
	} else if t, ok := os.LookupEnv("RUNE_TOKEN"); ok {
		options.Token = t
	}

	// Create the client
	return client.NewClient(options)
}

// InstanceResource is a wrapper around types.Instance that implements the Resource interface
type InstanceResource struct {
	*types.Instance
}

// GetKey returns a unique identifier for the instance
func (i InstanceResource) GetKey() string {
	return fmt.Sprintf("%s/%s", i.Namespace, i.ID)
}

// Equals checks if two instances are functionally equivalent for watch purposes
func (i InstanceResource) Equals(other Resource) bool {
	otherInstance, ok := other.(InstanceResource)
	if !ok {
		return false
	}

	// Check key fields that would make an instance visibly different in the table
	return i.ID == otherInstance.ID &&
		i.Name == otherInstance.Name &&
		i.Namespace == otherInstance.Namespace &&
		i.ServiceID == otherInstance.ServiceID &&
		i.NodeID == otherInstance.NodeID &&
		i.Status == otherInstance.Status
}

// InstanceWatcherAdapter adapts the InstanceWatcher interface to ResourceToWatch
type InstanceWatcherAdapter struct {
	InstanceWatcher
}

// Watch implements the ResourceToWatch interface
func (a InstanceWatcherAdapter) Watch(ctx context.Context, namespace, labelSelector, fieldSelector string) (<-chan WatchEvent, error) {
	// Parse any service filter from fieldSelector
	serviceID := ""
	fieldSelectorMap, err := parseSelector(fieldSelector)
	if err == nil {
		if svc, exists := fieldSelectorMap["service"]; exists {
			serviceID = svc
			// Remove from field selector to avoid double filtering
			delete(fieldSelectorMap, "service")
			// Rebuild field selector string
			var pairs []string
			for k, v := range fieldSelectorMap {
				pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
			}
			fieldSelector = strings.Join(pairs, ",")
		}
	}

	instCh, err := a.WatchInstances(namespace, serviceID, labelSelector, fieldSelector)
	if err != nil {
		return nil, err
	}

	// Create a new channel and adapt the events
	eventCh := make(chan WatchEvent)
	go func() {
		defer close(eventCh)

		for event := range instCh {
			if event.Error != nil {
				eventCh <- WatchEvent{
					Error: event.Error,
				}
				continue
			}

			// Convert instance to InstanceResource
			instResource := InstanceResource{event.Instance}

			eventCh <- WatchEvent{
				Resource:  instResource,
				EventType: event.EventType,
				Error:     nil,
			}
		}
	}()

	return eventCh, nil
}

// DefaultInstanceResourceToRows returns a default row renderer for instances
func DefaultInstanceResourceToRows(resources []Resource) [][]string {
	// First create a list of just the instances
	instances := make([]*types.Instance, 0, len(resources))
	for _, res := range resources {
		instRes, ok := res.(InstanceResource)
		if !ok {
			continue
		}
		instances = append(instances, instRes.Instance)
	}

	// Sort instances by name
	sort.Slice(instances, func(i, j int) bool {
		return instances[i].Name < instances[j].Name
	})

	// Build rows
	var rows [][]string

	// Add header row - but first check if we have any instances
	allNamespaces := false
	if len(instances) > 0 {
		for _, inst := range instances {
			if inst.Namespace != instances[0].Namespace {
				allNamespaces = true
				break
			}
		}
	}

	// Add header row
	if allNamespaces {
		rows = append(rows, []string{"NAMESPACE", "NAME", "SERVICE", "NODE", "STATUS", "RESTARTS", "AGE"})
	} else {
		rows = append(rows, []string{"NAME", "SERVICE", "NODE", "STATUS", "RESTARTS", "AGE"})
	}

	// Add instance rows
	for _, instance := range instances {
		// Format status using the same colorizeStatus function from table.go
		status := format.PTermStatusLabel(string(instance.Status))

		// Format restarts (currently a placeholder as we don't track this yet)
		restarts := "0" // Placeholder

		// Calculate age
		age := formatAge(instance.CreatedAt)

		// Create the row
		var row []string
		if allNamespaces {
			row = []string{
				instance.Namespace,
				instance.Name,
				instance.ServiceID,
				instance.NodeID,
				status,
				restarts,
				age,
			}
		} else {
			row = []string{
				instance.Name,
				instance.ServiceID,
				instance.NodeID,
				status,
				restarts,
				age,
			}
		}

		rows = append(rows, row)
	}

	return rows
}

// DefaultInstanceEventRenderer returns a default renderer for instance events
func DefaultInstanceEventRenderer(events []Event) []string {
	var lines []string
	for _, event := range events {
		var symbol, color string
		var eventPrefix string

		switch event.EventType {
		case "ADDED":
			symbol = "+"
			color = format.Green
			eventPrefix = "ADDED"
		case "MODIFIED":
			symbol = "~"
			color = format.Yellow
			eventPrefix = "MODIFIED"
		case "DELETED":
			symbol = "-"
			color = format.Red
			eventPrefix = "DELETED"
		}

		instRes, ok := event.Resource.(InstanceResource)
		if !ok {
			continue
		}

		eventText := fmt.Sprintf("[%s] %s instance \"%s\"",
			format.Colorize(color, symbol),
			eventPrefix,
			format.Colorize(format.Bold, instRes.Name))

		lines = append(lines, eventText)
	}
	return lines
}
