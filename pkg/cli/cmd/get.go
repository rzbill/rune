package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/pterm/pterm"
	"github.com/rzbill/rune/pkg/api/client"
	"github.com/rzbill/rune/pkg/cli/format"
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
	namespace := getNamespace
	if allNamespaces {
		namespace = "*" // Use wildcard to watch all namespaces
	}

	// Set up a channel to handle OS signals for graceful termination
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	// Create context with timeout if specified
	var cancel context.CancelFunc
	if watchTimeout != "" {
		duration, err := time.ParseDuration(watchTimeout)
		if err != nil {
			return fmt.Errorf("invalid timeout value: %w", err)
		}
		ctx, cancel = context.WithTimeout(ctx, duration)
	} else {
		// No timeout specified, create a cancellable context
		ctx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	// Create a filter for a specific service if provided
	var fieldSelectorStr string
	if resourceName != "" {
		fieldSelectorStr = fmt.Sprintf("name=%s", resourceName)
	}

	// Store services to track state
	services := make(map[string]*types.Service)

	// Store recent events
	recentEvents := make([]watchEvent, 0, 10)

	// Setup watch parameters
	initialSyncComplete := false
	initialSyncTimer := time.NewTimer(2 * time.Second)
	initialCaching := true
	initialBufferingTimeout := time.NewTimer(500 * time.Millisecond)
	reconnectDelay := 1 * time.Second
	maxReconnectDelay := 30 * time.Second
	lastUpdateTime := time.Now()

	// Status message for showing connection status
	statusMessage := ""

	// Clear the screen first
	fmt.Print("\033[H\033[2J")

	// Setup pterm components
	area, err := pterm.DefaultArea.WithRemoveWhenDone(true).Start()
	if err != nil {
		return fmt.Errorf("failed to start interactive area: %w", err)
	}
	defer area.Stop()

	// Start watching for service changes
	watchCh, err := serviceClient.WatchServices(namespace, labelSelector, fieldSelectorStr)
	if err != nil {
		return fmt.Errorf("failed to watch services: %w", err)
	}

	// Main refresh ticker
	refreshTicker := time.NewTicker(1 * time.Second)
	defer refreshTicker.Stop()

	// Render initial screen
	renderScreen(area, services, recentEvents, lastUpdateTime, initialSyncComplete, statusMessage)

	for {
		select {
		case <-initialBufferingTimeout.C:
			// Mark initial buffering as complete after timeout
			if initialCaching {
				initialCaching = false
				// Trigger an immediate refresh to show all services we've received so far
				renderScreen(area, services, recentEvents, lastUpdateTime, initialSyncComplete, statusMessage)
			}

		case <-initialSyncTimer.C:
			// Mark initial sync as complete after the timer expires
			initialSyncComplete = true

		case event, ok := <-watchCh:
			if !ok {
				// Channel closed, try to reconnect
				statusMessage = "Connection lost. Reconnecting..."
				renderScreen(area, services, recentEvents, lastUpdateTime, initialSyncComplete, statusMessage)
				time.Sleep(reconnectDelay)

				// Exponential backoff for reconnect attempts
				reconnectDelay = min(reconnectDelay*2, maxReconnectDelay)

				// Attempt to reestablish watch
				watchCh, err = serviceClient.WatchServices(namespace, labelSelector, fieldSelectorStr)
				if err != nil {
					statusMessage = fmt.Sprintf("Failed to reconnect: %v. Retrying in %v...", err, reconnectDelay)
					renderScreen(area, services, recentEvents, lastUpdateTime, initialSyncComplete, statusMessage)
					continue
				}

				// Reset delay on successful reconnection
				reconnectDelay = 1 * time.Second
				lastUpdateTime = time.Now()
				statusMessage = "Reconnected successfully"
				renderScreen(area, services, recentEvents, lastUpdateTime, initialSyncComplete, statusMessage)
				continue
			}

			if event.Error != nil {
				// Check if it's a timeout error
				if strings.Contains(event.Error.Error(), "DeadlineExceeded") ||
					strings.Contains(event.Error.Error(), "context deadline exceeded") ||
					strings.Contains(event.Error.Error(), "transport is closing") {
					// Handle timeout by reconnecting
					statusMessage = "Connection timed out. Reconnecting..."
					renderScreen(area, services, recentEvents, lastUpdateTime, initialSyncComplete, statusMessage)
					time.Sleep(reconnectDelay)

					// Exponential backoff for reconnect attempts
					reconnectDelay = min(reconnectDelay*2, maxReconnectDelay)

					// Attempt to reestablish watch
					watchCh, err = serviceClient.WatchServices(namespace, labelSelector, fieldSelectorStr)
					if err != nil {
						statusMessage = fmt.Sprintf("Failed to reconnect: %v. Retrying in %v...", err, reconnectDelay)
						renderScreen(area, services, recentEvents, lastUpdateTime, initialSyncComplete, statusMessage)
						continue
					}

					// Reset delay on successful reconnection
					reconnectDelay = 1 * time.Second
					lastUpdateTime = time.Now()
					statusMessage = "Reconnected successfully"
					renderScreen(area, services, recentEvents, lastUpdateTime, initialSyncComplete, statusMessage)
					continue
				}

				// For other errors, return the error
				return fmt.Errorf("watch error: %w", event.Error)
			}

			service := event.Service
			serviceKey := fmt.Sprintf("%s/%s", service.Namespace, service.Name)
			lastUpdateTime = time.Now()

			// Clear status message on successful event
			statusMessage = ""

			// Update our local state based on event type
			switch event.EventType {
			case "ADDED", "MODIFIED":
				oldService, exists := services[serviceKey]
				services[serviceKey] = service

				// Only add event for real changes or new additions after initial sync
				if !exists {
					if initialSyncComplete {
						// This is a genuinely new service after initial sync
						recentEvents = append(recentEvents, watchEvent{
							eventType: "ADDED",
							service:   service,
							timestamp: time.Now(),
						})
					}
				} else if event.EventType == "MODIFIED" && !serviceEqual(oldService, service) {
					// Always show modifications
					recentEvents = append(recentEvents, watchEvent{
						eventType: "MODIFIED",
						service:   service,
						timestamp: time.Now(),
					})
				}
			case "DELETED":
				delete(services, serviceKey)

				// Always show deletions
				recentEvents = append(recentEvents, watchEvent{
					eventType: "DELETED",
					service:   service,
					timestamp: time.Now(),
				})
			}

			// Limit recent events to last 10
			if len(recentEvents) > 10 {
				recentEvents = recentEvents[len(recentEvents)-10:]
			}

			// Only refresh if we're not in initial caching phase
			if !initialCaching {
				renderScreen(area, services, recentEvents, lastUpdateTime, initialSyncComplete, statusMessage)
			}

			// Reset reconnect delay on successful events
			reconnectDelay = 1 * time.Second

		case <-refreshTicker.C:
			// Only refresh if we've exited initial caching or if we have a good batch of services
			if !initialCaching || len(services) > 0 {
				renderScreen(area, services, recentEvents, lastUpdateTime, initialSyncComplete, statusMessage)
				// If we were caching, mark as no longer caching
				initialCaching = false
			}

		case <-sig:
			area.Update(pterm.Sprintf("\nWatch interrupted"))
			return nil

		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				area.Update(pterm.Sprintf("\nWatch timeout reached"))
				return nil
			}
			return ctx.Err()
		}
	}
}

// renderScreen uses pterm to render the current state
func renderScreen(area *pterm.AreaPrinter, services map[string]*types.Service, events []watchEvent, lastUpdate time.Time, initialSyncComplete bool, statusMessage string) {
	var content strings.Builder

	// Convert map to slice for sorting
	servicesList := make([]*types.Service, 0, len(services))
	for _, svc := range services {
		servicesList = append(servicesList, svc)
	}

	// Sort services by name
	sort.Slice(servicesList, func(i, j int) bool {
		return servicesList[i].Name < servicesList[j].Name
	})

	// Create service table
	if len(servicesList) > 0 {
		// Define column headers
		var headers []string
		if allNamespaces {
			headers = []string{"NAMESPACE", "NAME", "STATUS", "REPLICAS", "IMAGE/COMMAND", "AGE"}
		} else {
			headers = []string{"NAME", "STATUS", "REPLICAS", "IMAGE/COMMAND", "AGE"}
		}

		// Calculate column widths for proper formatting
		colWidths := make([]int, len(headers))
		for i, h := range headers {
			colWidths[i] = len(h)
		}

		// Find maximum width for each column
		for _, service := range servicesList {
			// Prepare row data
			var row []string

			// Format status
			status := string(service.Status)

			// Format instances
			instances := fmt.Sprintf("%d/%d", countRunningInstances(service), service.Scale)

			// Determine image or command
			imageOrCommand := service.Image
			if service.Runtime == "process" && service.Process != nil {
				imageOrCommand = service.Process.Command
				if len(service.Process.Args) > 0 {
					imageOrCommand += " " + strings.Join(service.Process.Args, " ")
				}
			}

			// Truncate very long image/command strings
			if len(imageOrCommand) > 60 {
				imageOrCommand = truncateString(imageOrCommand, 60)
			}

			// Calculate age
			age := formatAge(service.CreatedAt)

			// Create the row for width calculation
			if allNamespaces {
				row = []string{
					service.Namespace,
					service.Name,
					status,
					instances,
					imageOrCommand,
					age,
				}
			} else {
				row = []string{
					service.Name,
					status,
					instances,
					imageOrCommand,
					age,
				}
			}

			// Update column widths
			for i, cell := range row {
				if len(cell) > colWidths[i] {
					colWidths[i] = len(cell)
				}
			}
		}

		// Draw top border
		content.WriteString("┌")
		for i, width := range colWidths {
			content.WriteString(strings.Repeat("─", width+2))
			if i < len(colWidths)-1 {
				content.WriteString("┬")
			}
		}
		content.WriteString("┐\n")

		// Draw headers
		if !noHeaders {
			content.WriteString("│")
			for i, header := range headers {
				paddedHeader := fmt.Sprintf(" %s%s ",
					pterm.FgLightCyan.Sprint(header),
					strings.Repeat(" ", colWidths[i]-len(header)))
				content.WriteString(paddedHeader + "│")
			}
			content.WriteString("\n")

			// Draw header separator
			content.WriteString("├")
			for i, width := range colWidths {
				content.WriteString(strings.Repeat("─", width+2))
				if i < len(colWidths)-1 {
					content.WriteString("┼")
				}
			}
			content.WriteString("┤\n")
		}

		// Draw rows
		for _, service := range servicesList {
			// Format status with color
			status := string(service.Status)
			var statusColored string
			switch status {
			case string(types.ServiceStatusRunning):
				statusColored = pterm.FgGreen.Sprint(status)
			case string(types.ServiceStatusPending), string(types.ServiceStatusUpdating):
				statusColored = pterm.FgYellow.Sprint(status)
			case string(types.ServiceStatusFailed):
				statusColored = pterm.FgRed.Sprint(status)
			default:
				statusColored = status
			}

			// Format instances
			instances := fmt.Sprintf("%d/%d", countRunningInstances(service), service.Scale)

			// Determine image or command
			imageOrCommand := service.Image
			if service.Runtime == "process" && service.Process != nil {
				imageOrCommand = service.Process.Command
				if len(service.Process.Args) > 0 {
					imageOrCommand += " " + strings.Join(service.Process.Args, " ")
				}
			}

			// Truncate very long image/command strings
			if len(imageOrCommand) > 60 {
				imageOrCommand = truncateString(imageOrCommand, 60)
			}

			// Calculate age
			age := formatAge(service.CreatedAt)

			// Create and format the row
			var row []string
			if allNamespaces {
				row = []string{
					service.Namespace,
					service.Name,
					statusColored,
					instances,
					imageOrCommand,
					age,
				}
			} else {
				row = []string{
					service.Name,
					statusColored,
					instances,
					imageOrCommand,
					age,
				}
			}

			// Draw the row
			content.WriteString("│")
			for i, cell := range row {
				// For colored text, we need to calculate visible length without ANSI codes
				visLen := len(stripAnsi(cell))
				paddedCell := fmt.Sprintf(" %s%s ",
					cell,
					strings.Repeat(" ", colWidths[i]-visLen))
				content.WriteString(paddedCell + "│")
			}
			content.WriteString("\n")
		}

		// Draw bottom border
		content.WriteString("└")
		for i, width := range colWidths {
			content.WriteString(strings.Repeat("─", width+2))
			if i < len(colWidths)-1 {
				content.WriteString("┴")
			}
		}
		content.WriteString("┘\n\n")
	} else {
		content.WriteString(pterm.FgYellow.Sprint("No services found\n\n"))
	}

	// Display status message if any
	if statusMessage != "" {
		content.WriteString(pterm.FgYellow.Sprintf("⚠ %s\n\n", statusMessage))
	}

	// Show events if there are any and initial sync is complete
	if len(events) > 0 && initialSyncComplete {
		for i := 0; i < len(events); i++ {
			event := events[i]
			var symbol string
			var colorFunc func(...interface{}) string

			switch event.eventType {
			case "ADDED":
				symbol = "+"
				colorFunc = pterm.FgGreen.Sprint
			case "MODIFIED":
				symbol = "~"
				colorFunc = pterm.FgYellow.Sprint
			case "DELETED":
				symbol = "-"
				colorFunc = pterm.FgRed.Sprint
			}

			eventText := pterm.Sprintf("[%s] %s service \"%s\"",
				colorFunc(symbol),
				event.eventType,
				pterm.Bold.Sprint(event.service.Name))

			content.WriteString(eventText + "\n")
		}
		content.WriteString("\n")
	}

	content.WriteString(pterm.FgGray.Sprint("Press Ctrl+C to exit."))

	// Update the area with the newly rendered content - clear previous content
	area.Clear()
	area.Update(content.String())
}

// watchEvent represents a service change event for display
type watchEvent struct {
	eventType string
	service   *types.Service
	timestamp time.Time
}

// serviceEqual checks if two services are functionally equivalent for watch purposes
func serviceEqual(a, b *types.Service) bool {
	if a == nil || b == nil {
		return a == b
	}

	// Check key fields that would make a service visibly different in the table
	return a.Name == b.Name &&
		a.Namespace == b.Namespace &&
		a.Status == b.Status &&
		a.Scale == b.Scale &&
		a.Image == b.Image &&
		a.Runtime == b.Runtime
}

// getTerminalSize returns the dimensions of the terminal
func getTerminalSize() (width, height int, err error) {
	// Default sizes if detection fails
	width, height = 80, 24

	// Try to get actual terminal size
	cmd := exec.Command("stty", "size")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	if err == nil {
		parts := strings.Split(strings.TrimSpace(string(out)), " ")
		if len(parts) == 2 {
			if h, err := strconv.Atoi(parts[0]); err == nil {
				height = h
			}
			if w, err := strconv.Atoi(parts[1]); err == nil {
				width = w
			}
		}
	}

	return width, height, err
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
	// TODO: Add more resource types here
	default:
		return fmt.Errorf("unsupported resource type for table output")
	}
}

// outputServicesTable outputs services in a formatted table
func outputServicesTable(services []*types.Service) error {
	if len(services) == 0 {
		fmt.Println("No services found")
		return nil
	}

	// Define column headers
	var headers []string
	if allNamespaces {
		headers = []string{"NAMESPACE", "NAME", "TYPE", "STATUS", "REPLICAS", "IMAGE/COMMAND", "AGE"}
	} else {
		headers = []string{"NAME", "TYPE", "STATUS", "REPLICAS", "IMAGE/COMMAND", "AGE"}
	}

	// Calculate column widths for proper alignment
	colWidths := []int{}
	for _, h := range headers {
		colWidths = append(colWidths, len(h))
	}

	// Determine maximum width needed for each column based on content
	for _, service := range services {
		// First column: name or namespace/name
		if allNamespaces {
			updateMaxWidth(&colWidths[0], len(service.Namespace))
			updateMaxWidth(&colWidths[1], len(service.Name))
		} else {
			updateMaxWidth(&colWidths[0], len(service.Name))
		}

		// Type column: container or process
		serviceType := "container"
		if service.Runtime == "process" && service.Process != nil {
			serviceType = "process"
		}
		if allNamespaces {
			updateMaxWidth(&colWidths[2], len(serviceType))
		} else {
			updateMaxWidth(&colWidths[1], len(serviceType))
		}

		// Status column
		status := string(service.Status)
		if allNamespaces {
			updateMaxWidth(&colWidths[3], len(status))
		} else {
			updateMaxWidth(&colWidths[2], len(status))
		}

		// Replicas column
		instances := fmt.Sprintf("%d/%d", countRunningInstances(service), service.Scale)
		if allNamespaces {
			updateMaxWidth(&colWidths[4], len(instances))
		} else {
			updateMaxWidth(&colWidths[3], len(instances))
		}

		// Image/Command column
		imageOrCommand := service.Image
		if service.Runtime == "process" && service.Process != nil {
			imageOrCommand = service.Process.Command
			if len(service.Process.Args) > 0 {
				imageOrCommand += " " + strings.Join(service.Process.Args, " ")
			}
		}
		// Truncate very long image/command strings
		if len(imageOrCommand) > 60 {
			imageOrCommand = truncateString(imageOrCommand, 60)
		}
		if allNamespaces {
			updateMaxWidth(&colWidths[5], len(imageOrCommand))
		} else {
			updateMaxWidth(&colWidths[4], len(imageOrCommand))
		}

		// Age column
		age := formatAge(service.CreatedAt)
		if allNamespaces {
			updateMaxWidth(&colWidths[6], len(age))
		} else {
			updateMaxWidth(&colWidths[5], len(age))
		}
	}

	// Print top border
	printTableBorder(colWidths, "top")

	// Print headers
	if !noHeaders {
		printTableRow(headers, colWidths, true)

		// Print header separator
		printTableBorder(colWidths, "middle")
	}

	// Print rows
	for _, service := range services {
		// Determine type (container or process)
		serviceType := "container"
		if service.Runtime == "process" && service.Process != nil {
			serviceType = "process"
		}

		// Determine image or command
		imageOrCommand := service.Image
		if service.Runtime == "process" && service.Process != nil {
			imageOrCommand = service.Process.Command
			if len(service.Process.Args) > 0 {
				imageOrCommand += " " + strings.Join(service.Process.Args, " ")
			}
		}

		// Truncate very long image/command strings for better table layout
		if len(imageOrCommand) > 60 {
			imageOrCommand = truncateString(imageOrCommand, 60)
		}

		// Format status with color
		status := format.StatusLabel(string(service.Status))

		// Format instances (e.g., "3/3" for running)
		instances := fmt.Sprintf("%d/%d", countRunningInstances(service), service.Scale)

		// Calculate age
		age := formatAge(service.CreatedAt)

		// Create the row
		var row []string
		if allNamespaces {
			row = []string{service.Namespace, service.Name, serviceType, status, instances, imageOrCommand, age}
		} else {
			row = []string{service.Name, serviceType, status, instances, imageOrCommand, age}
		}

		printTableRow(row, colWidths, false)
	}

	// Print bottom border
	printTableBorder(colWidths, "bottom")

	return nil
}

// printTableBorder prints a table border row
func printTableBorder(colWidths []int, borderType string) {
	// Characters for table borders
	var line string

	// Choose the right border characters based on type
	var left, mid, right, cross string

	switch borderType {
	case "top":
		left = "┌"
		mid = "┬"
		right = "┐"
		cross = "─"
	case "middle":
		left = "├"
		mid = "┼"
		right = "┤"
		cross = "─"
	case "bottom":
		left = "└"
		mid = "┴"
		right = "┘"
		cross = "─"
	}

	// Build the border line
	for i, width := range colWidths {
		if i == 0 {
			// First column starts with left corner
			line += left + strings.Repeat(cross, width+2)
		} else {
			// All other columns have a mid connector
			line += mid + strings.Repeat(cross, width+2)
		}
	}

	// Close with the right corner
	line += right

	fmt.Println(line)
}

// printTableRow prints a table row with properly aligned columns
func printTableRow(row []string, colWidths []int, isHeader bool) {
	var rowStr string = "│ "

	for i, cell := range row {
		// Format the cell content (add color for headers)
		formattedCell := cell
		if isHeader {
			formattedCell = format.Colorize(format.BoldCyan, cell)
		}

		// Calculate padding needed to align column content
		paddedCell := padRight(formattedCell, colWidths[i])

		// First column already has the initial border
		if i == 0 {
			rowStr += paddedCell + " │ "
		} else {
			rowStr += paddedCell + " │ "
		}
	}

	// Remove trailing space if needed
	if len(rowStr) > 0 && rowStr[len(rowStr)-1] == ' ' {
		rowStr = rowStr[:len(rowStr)-1]
	}

	fmt.Println(rowStr)
}

// padRight adds spaces to the right of a string to reach the specified length
func padRight(s string, width int) string {
	// For colored text, we need to account for invisible ANSI codes
	visibleLen := len(stripAnsi(s))
	if visibleLen >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visibleLen)
}

// updateMaxWidth updates the maximum width if the new width is larger
func updateMaxWidth(currentMax *int, width int) {
	if width > *currentMax {
		*currentMax = width
	}
}

// stripAnsi removes ANSI color codes from a string for accurate length calculation
func stripAnsi(s string) string {
	// Simple regex to strip ANSI color codes
	ansiRegex := regexp.MustCompile("\x1b\\[[0-9;]*m")
	return ansiRegex.ReplaceAllString(s, "")
}

// truncateString truncates a string to maxLen and adds "..." if it was truncated
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
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

// countRunningInstances counts running instances for a service
// This is a placeholder that should be replaced with actual instance counting logic
func countRunningInstances(service *types.Service) int {
	// This is a placeholder - in a real implementation, we would count
	// the actual running instances from the service's status
	if service.Status == types.ServiceStatusRunning {
		return service.Scale
	}
	return 0
}

// formatAge formats a time.Time as a human-readable age string
func formatAge(t time.Time) string {
	if t.IsZero() {
		return "Unknown"
	}

	duration := time.Since(t)
	if duration < time.Minute {
		return "Just now"
	} else if duration < time.Hour {
		minutes := int(duration.Minutes())
		return fmt.Sprintf("%dm", minutes)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		return fmt.Sprintf("%dh", hours)
	} else if duration < 30*24*time.Hour {
		days := int(duration.Hours() / 24)
		return fmt.Sprintf("%dd", days)
	} else if duration < 365*24*time.Hour {
		months := int(duration.Hours() / 24 / 30)
		return fmt.Sprintf("%dmo", months)
	}
	years := int(duration.Hours() / 24 / 365)
	return fmt.Sprintf("%dy", years)
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

// getColumnWidths calculates appropriate column widths based on headers and services
func getColumnWidths(headers []string, services []*types.Service) []int {
	// Determine number of columns
	numCols := len(headers)

	// Initialize with header widths
	colWidths := make([]int, numCols)
	for i, h := range headers {
		colWidths[i] = len(h)
	}

	// Check all services to find maximum widths needed
	for _, service := range services {
		// Prepare row data to check lengths
		var row []string

		// Format status with color (calculate visual length without ANSI codes)
		status := stripAnsi(format.StatusLabel(string(service.Status)))

		// Format instances
		instances := fmt.Sprintf("%d/%d", countRunningInstances(service), service.Scale)

		// Determine image or command
		imageOrCommand := service.Image
		if service.Runtime == "process" && service.Process != nil {
			imageOrCommand = service.Process.Command
			if len(service.Process.Args) > 0 {
				imageOrCommand += " " + strings.Join(service.Process.Args, " ")
			}
		}

		// Truncate very long image/command strings
		if len(imageOrCommand) > 60 {
			imageOrCommand = truncateString(imageOrCommand, 60)
		}

		// Calculate age
		age := formatAge(service.CreatedAt)

		// Build the row based on all-namespaces flag
		if allNamespaces {
			row = []string{
				service.Namespace,
				service.Name,
				status,
				instances,
				imageOrCommand,
				age,
			}
		} else {
			row = []string{
				service.Name,
				status,
				instances,
				imageOrCommand,
				age,
			}
		}

		// Update column widths as needed
		for i, cell := range row {
			cellWidth := len(cell)
			if cellWidth > colWidths[i] {
				colWidths[i] = cellWidth
			}
		}
	}

	return colWidths
}

// min returns the smaller of two time.Duration values
func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
