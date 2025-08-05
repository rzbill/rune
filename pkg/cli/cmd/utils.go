package cmd

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

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
