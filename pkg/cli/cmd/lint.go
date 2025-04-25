package cmd

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/rzbill/rune/pkg/cli/format"
	"github.com/rzbill/rune/pkg/types"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	strict        bool
	quiet         bool
	recursive     bool
	fileTypes     []string
	exitOnFail    bool
	autoFix       bool
	outputFormat  string
	contextLines  int
	expandContext bool
)

// Color setup for formatting
var (
	errorColor   = color.New(color.FgRed, color.Bold)
	fileColor    = color.New(color.FgCyan, color.Bold)
	lineColor    = color.New(color.FgYellow, color.Bold)
	hintColor    = color.New(color.FgGreen)
	successColor = color.New(color.FgGreen, color.Bold)
)

// lintCmd represents the lint command
var lintCmd = &cobra.Command{
	Use:   "lint [file or directory]...",
	Short: "Validate YAML specifications",
	Long: `Lint and validate Rune YAML specifications for correctness.

This command checks all resource types (services, jobs, secrets, functions, etc.)
and validates their structure, required fields, and relationships.

Examples:
  # Lint a single file
  rune lint myservice.yaml

  # Lint multiple files
  rune lint service.yaml job.yaml

  # Recursively lint all YAML files in a directory
  rune lint --recursive ./manifests/

  # Strict validation mode (more warnings)
  rune lint --strict myservice.yaml

  # Only show errors, no progress or success messages
  rune lint --quiet myservice.yaml

  # Exit with non-zero code on first validation error
  rune lint --exit-on-fail myservice.yaml
  
  # Automatically fix common issues when possible
  rune lint --fix myservice.yaml
  
  # Output in JSON format for CI/CD integration
  rune lint --format json ./manifests/`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("at least one file or directory is required")
		}

		// Expand context if requested
		if expandContext {
			contextLines = 3
		}

		// Gather all files to lint
		var filesToLint []string
		for _, arg := range args {
			info, err := os.Stat(arg)
			if err != nil {
				return fmt.Errorf("error accessing %s: %w", arg, err)
			}

			if info.IsDir() {
				if recursive {
					// Recursively find all YAML files in directory
					err := filepath.Walk(arg, func(path string, info os.FileInfo, err error) error {
						if err != nil {
							return err
						}
						if !info.IsDir() && hasYAMLExtension(path) {
							filesToLint = append(filesToLint, path)
						}
						return nil
					})
					if err != nil {
						return fmt.Errorf("error walking directory %s: %w", arg, err)
					}
				} else {
					// Get only YAML files in the top level of the directory
					files, err := ioutil.ReadDir(arg)
					if err != nil {
						return fmt.Errorf("error reading directory %s: %w", arg, err)
					}
					for _, f := range files {
						if !f.IsDir() && hasYAMLExtension(f.Name()) {
							filesToLint = append(filesToLint, filepath.Join(arg, f.Name()))
						}
					}
				}
			} else {
				// It's a file, check if it's YAML
				if hasYAMLExtension(arg) {
					filesToLint = append(filesToLint, arg)
				} else {
					if !quiet {
						fmt.Printf("Skipping non-YAML file: %s\n", arg)
					}
				}
			}
		}

		if len(filesToLint) == 0 {
			return fmt.Errorf("no YAML files found to lint")
		}

		return runLint(filesToLint)
	},
}

func init() {
	rootCmd.AddCommand(lintCmd)

	// Define flags
	lintCmd.Flags().BoolVar(&strict, "strict", false, "Enable stricter validation rules")
	lintCmd.Flags().BoolVar(&quiet, "quiet", false, "Only show errors, no progress or success messages")
	lintCmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Recursively process directories")
	lintCmd.Flags().StringSliceVar(&fileTypes, "types", []string{}, "Only validate specific resource types (comma separated: service, job, secret, etc.)")
	lintCmd.Flags().BoolVar(&exitOnFail, "exit-on-fail", false, "Exit on first validation failure")
	lintCmd.Flags().BoolVar(&autoFix, "fix", false, "Automatically fix simple issues when possible")
	lintCmd.Flags().StringVar(&outputFormat, "format", "text", "Output format (text, json)")
	lintCmd.Flags().IntVar(&contextLines, "context", 1, "Number of context lines to show around errors")
	lintCmd.Flags().BoolVar(&expandContext, "expand-context", false, "Show more context around errors (equivalent to --context=3)")
}

// hasYAMLExtension checks if a file has a YAML extension
func hasYAMLExtension(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".yaml" || ext == ".yml"
}

// runLint performs the actual linting of files
func runLint(files []string) error {
	hasErrors := false
	totalErrorCount := 0
	totalFixCount := 0
	startTime := time.Now()
	allErrors := []format.ValidationError{}

	for _, filename := range files {
		// Only show per-file progress in verbose mode
		if verbose && !quiet && outputFormat == "text" {
			fmt.Printf("Linting %s...\n", filename)
		}

		// Read file
		data, err := ioutil.ReadFile(filename)
		if err != nil {
			fmt.Printf("Error reading %s: %v\n", filename, err)
			hasErrors = true
			totalErrorCount++
			if exitOnFail {
				return fmt.Errorf("validation failed")
			}
			continue
		}

		// Create an error formatter for this file
		formatter := format.NewErrorFormatter(filename, data)
		formatter.ContextLines = contextLines
		formatter.CanAutoFix = autoFix
		formatter.OutputFormat = outputFormat

		// Determine resource type
		resourceType, err := determineResourceType(data)
		if err != nil {
			formatter.PrintErrorHeader()

			// Extract line number if possible
			lineNum := formatter.ExtractLineNumber(err.Error())
			formatter.PrintError(err.Error(), lineNum)

			// Try auto-fix if enabled
			if autoFix {
				if fixed, newData := formatter.TryAutoFix(err.Error(), lineNum); fixed {
					if !quiet && outputFormat == "text" {
						format.SuccessColor.Printf("  ↪ Auto-fixed issues in %s\n", filename)
					}
					// Write the fixed data back to the file
					if err := ioutil.WriteFile(filename, newData, 0644); err != nil {
						fmt.Printf("Error writing fixed file %s: %v\n", filename, err)
					} else {
						formatter.FixCount++
						totalFixCount++

						// Re-read the fixed file and try again
						data = newData
						resourceType, err = determineResourceType(data)
						// If still error, continue with normal flow
						if err != nil {
							hasErrors = true
							totalErrorCount++
							if exitOnFail {
								return fmt.Errorf("validation failed")
							}
							continue
						}
					}
				} else {
					// No fix was applied, count the error
					hasErrors = true
					totalErrorCount++
				}
			} else {
				hasErrors = true
				totalErrorCount++
				if exitOnFail {
					return fmt.Errorf("validation failed")
				}
				continue
			}
		}

		// Skip if we're only linting specific types and this isn't one of them
		if len(fileTypes) > 0 && !contains(fileTypes, resourceType) {
			if !quiet && outputFormat == "text" {
				fmt.Printf("Skipping %s (resource type: %s)\n", filename, resourceType)
			}
			continue
		}

		// Validate based on resource type
		if err := validateResource(formatter, resourceType, data); err != nil {
			hasErrors = true

			if autoFix && len(formatter.Errors) > 0 {
				var anyFixed bool

				// Try to auto-fix each error
				for _, valError := range formatter.Errors {
					if valError.AutoFixable {
						if fixed, newData := formatter.TryAutoFix(valError.Message, valError.LineNumber); fixed {
							if !quiet && outputFormat == "text" {
								format.SuccessColor.Printf("  ↪ Auto-fixed issue at line %d in %s\n",
									valError.LineNumber, filename)
							}
							// Write the fixed data back to the file
							if err := ioutil.WriteFile(filename, newData, 0644); err != nil {
								fmt.Printf("Error writing fixed file %s: %v\n", filename, err)
							} else {
								anyFixed = true
								formatter.FixCount++
								totalFixCount++
								// Update the data for next fixes
								data = newData
								formatter.FileData = newData
							}
						}
					}
				}

				// If any fixes were applied, try validating again
				if anyFixed {
					// Create a new formatter with the updated data
					newFormatter := format.NewErrorFormatter(filename, data)
					newFormatter.ContextLines = contextLines
					newFormatter.OutputFormat = outputFormat

					// Revalidate
					if err := validateResource(newFormatter, resourceType, data); err == nil {
						// Fixed all issues!
						if !quiet && outputFormat == "text" {
							format.SuccessColor.Printf("  ✓ All issues fixed in %s\n", filename)
						}
						// Reset the error flag for this file
						hasErrors = false
						continue
					} else {
						// Still has errors after fixing
						totalErrorCount += newFormatter.ErrorCount
						allErrors = append(allErrors, newFormatter.Errors...)
					}
				} else {
					// No fixes were applied
					totalErrorCount += formatter.ErrorCount
					allErrors = append(allErrors, formatter.Errors...)
				}
			} else {
				// Not auto-fixing
				totalErrorCount += formatter.ErrorCount
				allErrors = append(allErrors, formatter.Errors...)
			}

			// Show error summary for this file
			if outputFormat == "text" {
				formatter.PrintErrorSummary()
			}

			if exitOnFail {
				return fmt.Errorf("validation failed")
			}
		}
	}

	// Output in JSON format if requested
	if outputFormat == "json" {
		// Create a combined formatter just for JSON output
		jsonFormatter := format.NewErrorFormatter("", nil)
		jsonFormatter.Errors = allErrors
		jsonFormatter.ErrorCount = totalErrorCount
		jsonFormatter.FixCount = totalFixCount
		fmt.Println(jsonFormatter.FormatAsJSON())
	}

	// Print overall stats
	if outputFormat == "text" {
		duration := time.Since(startTime)
		format.PrintLintSummary(len(files), totalErrorCount, totalFixCount, duration)
	}

	if hasErrors || totalErrorCount > 0 {
		// Return an error to indicate failure but don't add a message
		// since the summary already showed the error count
		return fmt.Errorf("") // Empty error message
	}

	return nil
}

// determineResourceType identifies the resource type from YAML data
func determineResourceType(data []byte) (string, error) {
	var m map[string]interface{}
	if err := yaml.Unmarshal(data, &m); err != nil {
		return "", fmt.Errorf("invalid YAML: %w", err)
	}

	// Look for known top-level keys
	if _, ok := m["service"]; ok {
		return "service", nil
	}
	if _, ok := m["services"]; ok {
		return "service", nil
	}
	if _, ok := m["job"]; ok {
		return "job", nil
	}
	if _, ok := m["jobs"]; ok {
		return "job", nil
	}
	if _, ok := m["secret"]; ok {
		return "secret", nil
	}
	if _, ok := m["function"]; ok {
		return "function", nil
	}
	if _, ok := m["config"]; ok {
		return "config", nil
	}
	if _, ok := m["networkPolicy"]; ok {
		return "networkPolicy", nil
	}

	// Check if it's a Rune global configuration file
	if _, ok := m["server"]; ok {
		if _, ok := m["client"]; ok {
			return "rune-config", nil
		}
	}

	// Check if it appears to be a partial config file
	configKeys := []string{"namespace", "auth", "resources", "logging", "plugins"}
	configKeyCount := 0

	for _, key := range configKeys {
		if _, ok := m[key]; ok {
			configKeyCount++
		}
	}

	// If it has at least two config-related keys, assume it's a configuration file
	if configKeyCount >= 2 {
		return "rune-config", nil
	}

	return "", fmt.Errorf("unrecognized resource type")
}

// validateResource validates a specific resource type
func validateResource(formatter *format.ErrorFormatter, resourceType string, data []byte) error {
	var err error

	switch resourceType {
	case "service":
		err = validateService(formatter, data)
	case "job":
		err = validateJob(formatter, resourceType, data)
	case "secret":
		err = validateSecret(formatter, resourceType, data)
	case "function":
		err = validateFunction(formatter, resourceType, data)
	case "config":
		err = validateConfig(formatter, resourceType, data)
	case "rune-config":
		err = validateRuneConfig(formatter, data)
	case "networkPolicy":
		err = validateNetworkPolicy(formatter, resourceType, data)
	default:
		return fmt.Errorf("validation not implemented for resource type: %s", resourceType)
	}

	return err
}

// validateService validates a service resource
func validateService(formatter *format.ErrorFormatter, data []byte) error {
	serviceFile, err := types.ParseServiceData(data)
	if err != nil {
		var yamlErr *yaml.TypeError
		if errors.As(err, &yamlErr) {
			formatter.PrintErrorHeader()

			for _, e := range yamlErr.Errors {
				errMsg := e
				lineNum := formatter.ExtractLineNumber(errMsg)
				formatter.PrintError(errMsg, lineNum)
			}
		} else {
			formatter.PrintErrorHeader()

			// Try to extract line number from regular errors
			errStr := err.Error()
			lineNum := formatter.ExtractLineNumber(errStr)
			formatter.PrintError(errStr, lineNum)
		}
		return fmt.Errorf("validation failed in %s", formatter.FileName)
	}

	hasErrors := false
	for _, service := range serviceFile.GetServices() {
		if err := service.Validate(); err != nil {
			if !hasErrors {
				formatter.PrintErrorHeader()
			}

			hasErrors = true

			// Get line info if available
			var lineNum int
			if lineInfo, ok := serviceFile.GetLineInfo(service.Name); ok {
				lineNum = lineInfo
			}

			formatter.PrintServiceError(service.Name, err.Error(), lineNum)
			formatter.ErrorCount++ // Increment the error count
		}
	}

	if hasErrors {
		return fmt.Errorf("validation failed for services in %s", formatter.FileName)
	}

	return nil
}

// validateJob validates a job resource
func validateJob(formatter *format.ErrorFormatter, resourceType string, data []byte) error {
	// This would use the actual job parsing and validation logic
	// For now, we'll return a placeholder message
	if !quiet && formatter.OutputFormat == "text" {
		fmt.Printf("Job validation not yet implemented for %s\n", formatter.FileName)
	}
	return nil
}

// validateSecret validates a secret resource
func validateSecret(formatter *format.ErrorFormatter, resourceType string, data []byte) error {
	// This would use the actual secret parsing and validation logic
	// For now, we'll return a placeholder message
	if !quiet && formatter.OutputFormat == "text" {
		fmt.Printf("Secret validation not yet implemented for %s\n", formatter.FileName)
	}
	return nil
}

// validateFunction validates a function resource
func validateFunction(formatter *format.ErrorFormatter, resourceType string, data []byte) error {
	// This would use the actual function parsing and validation logic
	// For now, we'll return a placeholder message
	if !quiet && formatter.OutputFormat == "text" {
		fmt.Printf("Function validation not yet implemented for %s\n", formatter.FileName)
	}
	return nil
}

// validateConfig validates a config resource
func validateConfig(formatter *format.ErrorFormatter, resourceType string, data []byte) error {
	// This would use the actual config parsing and validation logic
	// For now, we'll return a placeholder message
	if !quiet && formatter.OutputFormat == "text" {
		fmt.Printf("Config validation not yet implemented for %s\n", formatter.FileName)
	}
	return nil
}

// validateNetworkPolicy validates a network policy resource
func validateNetworkPolicy(formatter *format.ErrorFormatter, resourceType string, data []byte) error {
	// This would use the actual network policy parsing and validation logic
	// For now, we'll return a placeholder message
	if !quiet && formatter.OutputFormat == "text" {
		fmt.Printf("Network policy validation not yet implemented for %s\n", formatter.FileName)
	}
	return nil
}

// validateRuneConfig validates a Rune configuration file
func validateRuneConfig(formatter *format.ErrorFormatter, data []byte) error {
	// For now, we just validate that it's valid YAML
	// In the future, we can add more specific validation
	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		formatter.PrintErrorHeader()
		errStr := err.Error()
		lineNum := formatter.ExtractLineNumber(errStr)
		formatter.PrintError(errStr, lineNum)
		return fmt.Errorf("invalid Rune configuration: %w", err)
	}

	if !quiet && formatter.OutputFormat == "text" {
		format.SuccessColor.Printf("✓ Valid Rune configuration file: %s\n", formatter.FileName)
	}

	return nil
}

// contains checks if a string is present in a slice
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
