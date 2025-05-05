package cmd

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/pterm/pterm"
	"github.com/rzbill/rune/pkg/cli/format"
	"github.com/rzbill/rune/pkg/types"
)

// ResourceTable provides a generic interface for rendering tables of different resources
type ResourceTable struct {
	// Configuration
	Headers       []string
	ShowHeaders   bool
	AllNamespaces bool
	ShowLabels    bool
	MaxWidth      int

	// Rendering details
	tableRenderer *pterm.TablePrinter
	stripAnsiFunc func(string) string
}

// NewResourceTable creates a new resource table with default configuration
func NewResourceTable() *ResourceTable {
	// Create the table with custom header style
	table := pterm.DefaultTable.WithHasHeader(true)

	// Customize the header style to use BoldBlue
	headerStyle := pterm.NewStyle(pterm.FgCyan, pterm.Bold)
	table = table.WithHeaderStyle(headerStyle)

	return &ResourceTable{
		ShowHeaders:   true,
		stripAnsiFunc: stripAnsiTable,
		tableRenderer: table,
		MaxWidth:      100,
	}
}

// SetHeaders sets the headers for the table
func (t *ResourceTable) SetHeaders(headers []string) {
	t.Headers = headers
}

// SetStripAnsiFunc sets a custom function for stripping ANSI codes
func (t *ResourceTable) SetStripAnsiFunc(fn func(string) string) {
	t.stripAnsiFunc = fn
}

// RenderServices renders a table of services
func (t *ResourceTable) RenderServices(services []*types.Service) error {
	if len(services) == 0 {
		fmt.Println("No services found")
		return nil
	}

	// Set default headers if not provided
	if len(t.Headers) == 0 {
		if t.AllNamespaces {
			t.Headers = []string{"NAMESPACE", "NAME", "TYPE", "STATUS", "INSTANCES", "IMAGE/COMMAND", "AGE"}
		} else {
			t.Headers = []string{"NAME", "TYPE", "STATUS", "INSTANCES", "IMAGE/COMMAND", "AGE"}
		}
	}

	// Create rows
	rows := [][]string{t.Headers} // Start with headers

	// Generate data rows
	for _, service := range services {
		// Determine service type
		serviceType := "container"
		if service.Runtime == "process" && service.Process != nil {
			serviceType = "process"
		}

		// Format status - use our colorizeStatus function
		status := format.PTermStatusLabel(string(service.Status))

		// Format instances
		running := 0
		if service.Status == types.ServiceStatusRunning {
			running = service.Scale
		}
		instances := fmt.Sprintf("%d/%d", running, service.Scale)

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
			imageOrCommand = imageOrCommand[:57] + "..."
		}

		// Calculate age
		age := formatAgeTable(service.Metadata.CreatedAt)

		// Create the row
		var row []string
		if t.AllNamespaces {
			row = []string{
				service.Namespace,
				service.Name,
				serviceType,
				status,
				instances,
				imageOrCommand,
				age,
			}
		} else {
			row = []string{
				service.Name,
				serviceType,
				status,
				instances,
				imageOrCommand,
				age,
			}
		}

		// Add labels if requested
		if t.ShowLabels && len(service.Labels) > 0 {
			labelStrs := make([]string, 0, len(service.Labels))
			for k, v := range service.Labels {
				labelStrs = append(labelStrs, fmt.Sprintf("%s=%s", k, v))
			}
			row = append(row, strings.Join(labelStrs, ","))
		}

		rows = append(rows, row)
	}

	// Render the table with pterm
	return t.tableRenderer.WithData(rows).Render()
}

// RenderInstances renders a table of instances
func (t *ResourceTable) RenderInstances(instances []*types.Instance) error {
	if len(instances) == 0 {
		fmt.Println("No instances found")
		return nil
	}

	// Set default headers if not provided
	if len(t.Headers) == 0 {
		if t.AllNamespaces {
			t.Headers = []string{"NAMESPACE", "NAME", "SERVICE", "NODE", "STATUS", "RESTARTS", "AGE"}
		} else {
			t.Headers = []string{"NAME", "SERVICE", "NODE", "STATUS", "RESTARTS", "AGE"}
		}
	}

	// Create rows
	rows := [][]string{t.Headers} // Start with headers

	// Generate data rows
	for _, instance := range instances {
		// Format status using PTermStatusLabel
		status := format.PTermStatusLabel(string(instance.Status))

		// Format restarts (currently a placeholder as we don't track this yet)
		restarts := "0" // Placeholder

		// Calculate age
		age := formatAgeTable(instance.CreatedAt)

		// Create the row
		var row []string
		if t.AllNamespaces {
			row = []string{
				instance.Namespace,
				instance.Name,
				fmt.Sprintf("%s(%s)", instance.ServiceName, instance.ServiceID),
				instance.NodeID,
				status,
				restarts,
				age,
			}
		} else {
			row = []string{
				instance.Name,
				fmt.Sprintf("%s(%s)", instance.ServiceName, instance.ServiceID),
				instance.NodeID,
				status,
				restarts,
				age,
			}
		}

		rows = append(rows, row)
	}

	// Render the table with pterm
	return t.tableRenderer.WithData(rows).Render()
}

// RenderNamespaces renders a table of namespaces
// Note: This is a placeholder implementation that will need to be updated
// once the Namespace type is fully defined
func (t *ResourceTable) RenderNamespaces(namespaces []*types.Namespace) error {
	if len(namespaces) == 0 {
		fmt.Println("No namespaces found")
		return nil
	}

	// Set default headers if not provided
	if len(t.Headers) == 0 {
		t.Headers = []string{"NAME", "STATUS", "AGE"}
	}

	// Placeholder message
	fmt.Println("Namespace rendering not yet implemented")
	return nil
}

// Helper functions with unique names to avoid conflicts

// formatAgeTable formats a time.Time as a human-readable age string
func formatAgeTable(t time.Time) string {
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

// stripAnsiTable removes ANSI color codes from a string for accurate length calculation
func stripAnsiTable(s string) string {
	// Simple regex to strip ANSI color codes
	ansiRegex := regexp.MustCompile("\x1b\\[[0-9;]*m")
	return ansiRegex.ReplaceAllString(s, "")
}
