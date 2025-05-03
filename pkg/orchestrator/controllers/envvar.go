package controllers

import (
	"fmt"
	"strings"

	"github.com/rzbill/rune/pkg/types"
)

// prepareEnvVars prepares environment variables for an instance
func prepareEnvVars(service *types.Service, instance *types.Instance) map[string]string {
	envVars := make(map[string]string)

	// Add service-defined environment variables
	for key, value := range service.Env {
		envVars[key] = value
	}

	// Add built-in environment variables
	envVars["RUNE_SERVICE_NAME"] = service.Name
	envVars["RUNE_SERVICE_NAMESPACE"] = service.Namespace
	envVars["RUNE_INSTANCE_ID"] = instance.ID

	// Add normalized environment variables (for compatibility)
	serviceName := strings.ToUpper(service.Name)
	serviceName = strings.ReplaceAll(serviceName, "-", "_")

	envVars[fmt.Sprintf("%s_SERVICE_HOST", serviceName)] = fmt.Sprintf("%s.%s.rune", service.Name, service.Namespace)

	// Add port-related environment variables
	for _, port := range service.Ports {
		portName := strings.ToUpper(port.Name)
		portName = strings.ReplaceAll(portName, "-", "_")

		envVars[fmt.Sprintf("%s_SERVICE_PORT_%s", serviceName, portName)] = fmt.Sprintf("%d", port.Port)

		// If this is the first port, also set the default port
		if len(envVars[fmt.Sprintf("%s_SERVICE_PORT", serviceName)]) == 0 {
			envVars[fmt.Sprintf("%s_SERVICE_PORT", serviceName)] = fmt.Sprintf("%d", port.Port)
		}
	}

	return envVars
}
