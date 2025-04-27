package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Simple mock CLI for integration testing
func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: rune-test <command> [args...]")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "cast":
		handleCast(os.Args[2:])
	default:
		fmt.Printf("Unknown command: %s\n", command)
		os.Exit(1)
	}
}

func handleCast(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: rune-test cast <file> [--namespace=<namespace>] [--dry-run]")
		os.Exit(1)
	}

	var namespace = "default"
	var dryRun = false
	var filePath = ""

	// Parse args
	for _, arg := range args {
		if strings.HasPrefix(arg, "--namespace=") {
			namespace = strings.TrimPrefix(arg, "--namespace=")
		} else if arg == "--dry-run" {
			dryRun = true
		} else if !strings.HasPrefix(arg, "--") {
			filePath = arg
		}
	}

	fmt.Printf("Deploying service from file: %s\n", filePath)
	fmt.Printf("Namespace: %s\n", namespace)

	if dryRun {
		fmt.Printf("Service 'dry-run-test' would be created\n")
		return
	}

	// Read the service file
	data, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Error reading file: %s\n", err)
		os.Exit(1)
	}

	// Parse the service definition
	var serviceMap map[string]interface{}
	err = json.Unmarshal(data, &serviceMap)
	if err != nil {
		// If it's not JSON, assume it's YAML-like format
		lines := strings.Split(string(data), "\n")
		serviceMap = make(map[string]interface{})

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			parts := strings.SplitN(line, ":", 2)
			if len(parts) < 2 {
				continue
			}

			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			// Handle some special cases
			if key == "command" && strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
				// Parse as array
				cmdString := value[1 : len(value)-1]
				cmdParts := strings.Split(cmdString, ",")
				var cmds []string
				for _, cmd := range cmdParts {
					cmd = strings.Trim(cmd, " \"'")
					cmds = append(cmds, cmd)
				}
				serviceMap[key] = cmds
			} else {
				serviceMap[key] = value
			}
		}
	}

	// Create a service object for the test
	svcName, _ := serviceMap["name"].(string)
	if svcName == "" {
		fmt.Println("Service name not specified in file")
		os.Exit(1)
	}

	svcRuntime, _ := serviceMap["runtime"].(string)
	if svcRuntime == "" {
		fmt.Println("Service runtime not specified in file")
		os.Exit(1)
	}

	// Simple validation
	if svcRuntime != "container" && svcRuntime != "process" {
		fmt.Printf("Error: invalid runtime\n")
		os.Exit(1)
	}

	// Write to the test fixtures directory
	fixtureDir := os.Getenv("RUNE_FIXTURE_DIR")
	if fixtureDir == "" {
		fixtureDir = "test/integration/fixtures"
	}

	// Ensure the directory exists
	err = os.MkdirAll(fixtureDir, 0755)
	if err != nil {
		fmt.Printf("Error creating fixture directory: %s\n", err)
		os.Exit(1)
	}

	// Create the service file with namespace prefix
	fixturePath := fmt.Sprintf("%s/%s-%s.json", fixtureDir, namespace, svcName)

	// Add namespace to the service
	serviceMap["namespace"] = namespace

	// Write the file
	jsonData, _ := json.MarshalIndent(serviceMap, "", "  ")
	err = os.WriteFile(fixturePath, jsonData, 0644)
	if err != nil {
		fmt.Printf("Error writing fixture: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("Service '%s' created\n", svcName)
}
