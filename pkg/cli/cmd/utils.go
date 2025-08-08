package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/rzbill/rune/pkg/api/generated"
	"github.com/rzbill/rune/pkg/types"
	"gopkg.in/yaml.v3"
)

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
