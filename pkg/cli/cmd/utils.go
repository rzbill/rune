package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/rzbill/rune/internal/config"
	"github.com/rzbill/rune/pkg/api/generated"
	"github.com/rzbill/rune/pkg/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// resolveResourceType resolves a resource type from user input to a canonical type
func resolveResourceType(input string) (string, error) {
	// Lookup the resource type alias
	if resourceType, ok := resourceAliases[input]; ok {
		return resourceType, nil
	}

	return "", fmt.Errorf("unsupported resource type: %s", input)
}

// ensureDefaultGRPCPort ensures a server address includes a port, defaulting to types.DefaultGRPCPort for gRPC.
// Accepts either a bare host or an http/https URL; returns host:port without scheme.
// Examples:
//   - https://prod.example.com -> prod.example.com:7863
//   - example.com -> example.com:7863
//   - host:9000 -> host:9000 (unchanged)
func ensureDefaultGRPCPort(server string) string {
	if server == "" {
		return server
	}
	host := server
	if u, err := url.Parse(server); err == nil && u.Host != "" {
		host = u.Host
	}
	if _, _, err := netSplitHostPortSafe(host); err == nil {
		return host
	}
	if !strings.Contains(host, ":") {
		return fmt.Sprintf("%s:%d", host, config.DefaultGRPCPort)
	}
	return host
}

func getTargetRuneServer() string {
	// Determine target display name
	var targetDisplay string
	if s := viper.GetString("contexts.default.server"); s != "" {
		targetDisplay = ensureDefaultGRPCPort(s)
	} else {
		return fmt.Sprintf("localhost:%d", config.DefaultGRPCPort)
	}
	return targetDisplay
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

// outputTable outputs resources in table format
func outputTable(resources interface{}) error {
	switch r := resources.(type) {
	case []*types.Service:
		return outputServicesTable(r)
	case []*types.Instance:
		return outputInstancesTable(r)
	case []*types.Namespace:
		return outputNamespacesTable(r)
	case []*generated.DeletionOperation:
		return outputDeleteTable(r)
	case []*types.Secret:
		return outputSecretsTable(r)
	case []*types.Configmap:
		return outputConfigmapsTable(r)
	// TODO: Add more resource types here
	default:
		return fmt.Errorf("unsupported resource type for table output")
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

// netSplitHostPortSafe tries to split host:port but tolerates missing port
func netSplitHostPortSafe(hostport string) (host, port string, err error) {
	if strings.HasPrefix(hostport, "[") {
		// IPv6 literal
		if i := strings.LastIndex(hostport, "]"); i != -1 {
			host = hostport[:i+1]
			rest := hostport[i+1:]
			if strings.HasPrefix(rest, ":") {
				port = rest[1:]
				if port == "" {
					return "", "", fmt.Errorf("missing port")
				}
				return host, port, nil
			}
			return host, "", fmt.Errorf("no port")
		}
	}
	if i := strings.LastIndex(hostport, ":"); i != -1 && !strings.Contains(hostport[i+1:], ":") {
		return hostport[:i], hostport[i+1:], nil
	}
	return "", "", fmt.Errorf("no port")
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
