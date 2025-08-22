package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/rzbill/rune/pkg/api/client"
	"github.com/rzbill/rune/pkg/api/generated"
	"github.com/rzbill/rune/pkg/utils"
	"github.com/spf13/cobra"
)

// Valid output formats
const (
	OutputFormatText = "text"
	OutputFormatJSON = "json"
	OutputFormatRaw  = "raw"
)

var (
	// Logs command flags
	logsNamespace      string
	logsFollow         bool
	logsTail           int
	logsSinceStr       string
	logsUntilStr       string
	logsPattern        string
	logsShowTimestamps bool
	logsShowPrefix     bool
	logsNoColor        bool
	logsOutputFormat   string
	logsClientAddr     string
	logsInstanceID     string
)

// LogsOptions defines the configuration for log tracing
type LogsOptions struct {
	Follow         bool
	Tail           int
	Since          time.Time
	Until          time.Time
	Pattern        string
	ShowTimestamps bool
	ShowPrefix     bool
	NoColor        bool
	OutputFormat   string
	Namespace      string
	InstanceID     string
}

// logsCmd represents the logs command
var logsCmd = &cobra.Command{
	Use:   "logs (SERVICE_NAME | INSTANCE_NAME | TYPE/NAME)",
	Short: "Show logs for a service or instance",
	Long: `Display logs for Rune services and instances.

This command allows you to view the logs of your services, helping
with debugging, monitoring, and understanding service behavior.

Examples:
  # Stream logs from a service by name
  rune logs api

  # View logs from a specific instance by name
  rune logs api-instance-123

  # Use the explicit TYPE/NAME format
  rune logs service/api

  # Show only the last 50 lines of logs
  rune logs api --tail=50

  # Show logs from the last 10 minutes
  rune logs api --since=10m

  # Show logs from the last 10 minutes until 5 minutes ago
  rune logs api --since=10m --until=5m

  # Filter logs containing the word "error"
  rune logs api --pattern=error

  # Show timestamps in the output
  rune logs api --timestamps

  # Show resource type, service, and instance prefixes
  rune logs api --prefix

  # Output logs in JSON format for machine processing
  rune logs api --output=json`,
	Aliases:       []string{"log"},
	Args:          cobra.ExactArgs(1),
	RunE:          runLogs,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.AddCommand(logsCmd)

	// Local flags for the logs command
	logsCmd.Flags().StringVarP(&logsNamespace, "namespace", "n", "default", "Namespace of the service")
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Stream logs in real-time")
	logsCmd.Flags().IntVarP(&logsTail, "tail", "t", 100, "Number of recent log lines to show (use 0 for all available)")
	logsCmd.Flags().StringVar(&logsSinceStr, "since", "", "Show logs since timestamp (e.g., '5m', '2h', '2023-01-01T10:00:00Z')")
	logsCmd.Flags().StringVar(&logsUntilStr, "until", "", "Show logs until timestamp (e.g., '5m', '2h', '2023-01-01T10:00:00Z')")
	logsCmd.Flags().StringVarP(&logsPattern, "pattern", "p", "", "Filter logs by pattern")
	logsCmd.Flags().BoolVar(&logsShowTimestamps, "timestamps", false, "Show timestamps in the output")
	logsCmd.Flags().BoolVar(&logsShowPrefix, "prefix", false, "Show resource type, service and instance prefixes in log output")
	logsCmd.Flags().BoolVar(&logsNoColor, "no-color", false, "Disable colorized output")
	logsCmd.Flags().StringVarP(&logsOutputFormat, "output", "o", OutputFormatText, "Output format: text, json, or raw")

	// API client flags
	logsCmd.Flags().StringVar(&logsClientAddr, "api-server", "", "Address of the API server")
}

// runLogs is the main entry point for the logs command
func runLogs(cmd *cobra.Command, args []string) error {
	resourceArg := args[0]

	// Configure logs options
	options, err := parseLogsOptions()
	if err != nil {
		return err
	}

	// Create API client
	apiClient, err := newAPIClient(logsClientAddr, "")
	if err != nil {
		return fmt.Errorf("failed to connect to API server: %w", err)
	}
	defer apiClient.Close()

	// Set up context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle Ctrl+C gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Fprintln(os.Stderr, "\nReceived interrupt, shutting down...")
		cancel()
	}()

	return streamLogs(ctx, apiClient, resourceArg, options)
}

// parseLogsOptions parses and validates command line flags into TraceOptions
func parseLogsOptions() (*LogsOptions, error) {
	options := &LogsOptions{
		Follow:         logsFollow,
		Tail:           logsTail,
		Pattern:        logsPattern,
		ShowTimestamps: logsShowTimestamps,
		ShowPrefix:     logsShowPrefix,
		NoColor:        logsNoColor,
		OutputFormat:   logsOutputFormat,
		Namespace:      logsNamespace,
		InstanceID:     logsInstanceID,
	}

	// Validate output format
	switch options.OutputFormat {
	case OutputFormatText, OutputFormatJSON, OutputFormatRaw:
		// Valid format
	default:
		return nil, fmt.Errorf("invalid output format: %s (must be text, json, or raw)", options.OutputFormat)
	}

	// Parse since time if provided
	if logsSinceStr != "" {
		since, err := parseSinceTime(logsSinceStr)
		if err != nil {
			return nil, fmt.Errorf("invalid since time: %w", err)
		}
		options.Since = since
	}

	// Parse until time if provided
	if logsUntilStr != "" {
		// If "until" is a relative time, interpret it as time ago from now
		until, err := parseSinceTime(logsUntilStr)
		if err != nil {
			return nil, fmt.Errorf("invalid until time: %w", err)
		}
		options.Until = until
	}

	return options, nil
}

// parseSinceTime parses a human-friendly time string into a time.Time
func parseSinceTime(value string) (time.Time, error) {
	// Try as duration (e.g., "5m", "2h")
	if duration, err := time.ParseDuration(value); err == nil {
		return time.Now().Add(-duration), nil
	}

	// Try common timestamp formats
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}

	for _, layout := range formats {
		if t, err := time.Parse(layout, value); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unrecognized time format: %s", value)
}

// streamLogs streams logs from all instances of a service
func streamLogs(ctx context.Context, apiClient *client.Client, targetName string, options *LogsOptions) error {
	// Create a log client directly from the API client
	logClient := client.NewLogClient(apiClient)

	// Create stream using the convenience method
	stream, err := logClient.StreamLogs(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to log service: %w", err)
	}

	// Create log request for service
	logRequest := &generated.LogRequest{
		ResourceTarget: targetName,
		Namespace:      options.Namespace,
		Follow:         options.Follow,
		Tail:           utils.ToInt32NonNegative(options.Tail),
		Timestamps:     options.ShowTimestamps,
	}

	// Add common parameters to request
	addCommonRequestParams(logRequest, options)

	// Send the initial request
	if err := stream.Send(logRequest); err != nil {
		return fmt.Errorf("failed to send log request: %w", err)
	}

	// Handle logs based on mode (streaming or non-streaming)
	if !options.Follow {
		return handleNonStreamingLogs(ctx, stream, options)
	} else {
		return handleStreamingLogs(ctx, stream, options)
	}
}

// addCommonRequestParams adds common parameters to the log request
func addCommonRequestParams(logRequest *generated.LogRequest, options *LogsOptions) {
	// Add since time if specified
	if !options.Since.IsZero() {
		logRequest.Since = options.Since.Format(time.RFC3339)
	}

	// Add until time if specified
	if !options.Until.IsZero() {
		logRequest.Until = options.Until.Format(time.RFC3339)
	}

	// Add filter if specified
	if options.Pattern != "" {
		logRequest.Filter = options.Pattern
	}
}

// handleNonStreamingLogs handles logs in non-streaming (non-follow) mode
func handleNonStreamingLogs(ctx context.Context, stream generated.LogService_StreamLogsClient, options *LogsOptions) error {
	// Create a timeout context to prevent infinite waiting in non-follow mode
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Use a ring buffer approach to only keep the most recent logs
	// This is much more memory efficient than collecting all logs first
	ringBuffer := make([]*generated.LogResponse, options.Tail)
	bufferIndex := 0
	bufferFilled := false

	// Collect logs until we have enough, hit EOF, or timeout
	logsCollecting := true
	for logsCollecting {
		// Use a channel and goroutine to handle timeout gracefully
		respCh := make(chan *generated.LogResponse, 1)
		errCh := make(chan error, 1)

		go func() {
			r, e := stream.Recv()
			if e != nil {
				errCh <- e
				return
			}
			respCh <- r
		}()

		// Wait for either response, error, or timeout
		select {
		case resp := <-respCh:
			// Skip logs that don't match filters
			if !shouldProcessLog(resp, options) {
				continue
			}

			// Add to ring buffer, overwriting older entries as needed
			ringBuffer[bufferIndex] = resp
			bufferIndex = (bufferIndex + 1) % options.Tail
			if bufferIndex == 0 {
				bufferFilled = true
			}

			// If we've received enough logs and filled the buffer, we can exit collection
			if options.Tail > 0 && (bufferFilled || bufferIndex >= options.Tail) {
				logsCollecting = false
			}

		case err := <-errCh:
			// Check error type
			if err == io.EOF {
				// End of stream, proceed with processing collected logs
				logsCollecting = false
			} else {
				// Any other error should be returned
				return fmt.Errorf("error receiving logs: %w", err)
			}

		case <-timeoutCtx.Done():
			// Timeout reached, proceed with processing the logs we have
			logsCollecting = false
		}
	}

	// Process logs in the correct order
	var logsToProcess []*generated.LogResponse
	if bufferFilled {
		// Buffer has wrapped around, reorder to show oldest to newest
		logsToProcess = append(ringBuffer[bufferIndex:], ringBuffer[:bufferIndex]...)
	} else {
		// Buffer hasn't filled up yet, just take the valid entries
		logsToProcess = ringBuffer[:bufferIndex]
	}

	// Process the filtered logs
	for _, resp := range logsToProcess {
		if resp == nil {
			continue // Skip any nil entries in the buffer
		}
		if err := processLogResponse(resp, options); err != nil {
			fmt.Fprintf(os.Stderr, "Error processing log: %v\n", err)
		}
	}

	return nil
}

// handleStreamingLogs handles logs in streaming (follow) mode
func handleStreamingLogs(ctx context.Context, stream generated.LogService_StreamLogsClient, options *LogsOptions) error {
	errCh := make(chan error, 1)

	// For follow mode, process logs as they arrive
	go func() {
		defer close(errCh)

		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				// Stream closed normally
				errCh <- nil
				return
			}
			if err != nil {
				errCh <- fmt.Errorf("error receiving logs: %w", err)
				return
			}

			// Skip logs that don't match filters
			if !shouldProcessLog(resp, options) {
				continue
			}

			// Process the log response
			if err := processLogResponse(resp, options); err != nil {
				fmt.Fprintf(os.Stderr, "Error processing log: %v\n", err)
			}
		}
	}()

	// Wait for completion or error
	select {
	case <-ctx.Done():
		// Context canceled, clean up
		return nil
	case err := <-errCh:
		return err
	}
}

// shouldProcessLog determines if a log should be processed based on filters
func shouldProcessLog(resp *generated.LogResponse, options *LogsOptions) bool {
	// Apply pattern filter if specified
	if options.Pattern != "" && !strings.Contains(strings.ToLower(resp.Content), strings.ToLower(options.Pattern)) {
		return false
	}

	// Skip logs outside the time range if Until is specified
	if !options.Until.IsZero() {
		if logTime, err := time.Parse(time.RFC3339, resp.Timestamp); err == nil {
			if logTime.After(options.Until) {
				return false
			}
		}
	}

	return true
}

// processLogResponse processes and displays a log response based on the output format
func processLogResponse(resp *generated.LogResponse, options *LogsOptions) error {
	// Get the log level
	logLevel := resp.LogLevel

	// Fallback determination if the server doesn't provide this field
	if logLevel == "" {
		logLevel = "info"
		if strings.Contains(strings.ToLower(resp.Content), "error") {
			logLevel = "error"
		}
	}

	// Apply pattern filter if specified
	if options.Pattern != "" && !strings.Contains(strings.ToLower(resp.Content), strings.ToLower(options.Pattern)) {
		return nil
	}

	// Clean up the content - remove any special characters that might cause line breaks
	content := resp.Content
	// Replace any carriage returns or newlines with spaces
	content = strings.ReplaceAll(content, "\r", " ")
	content = strings.ReplaceAll(content, "\n", " ")
	// Normalize multiple spaces into single spaces
	content = strings.Join(strings.Fields(content), " ")

	// Set up colors for text output
	timeColor := color.New(color.FgCyan)
	nameColor := color.New(color.FgGreen, color.Bold)
	errorColor := color.New(color.FgRed)
	infoColor := color.New(color.FgWhite)

	if options.NoColor {
		timeColor.DisableColor()
		nameColor.DisableColor()
		errorColor.DisableColor()
		infoColor.DisableColor()
	}

	// Format timestamp if provided
	timestamp := resp.Timestamp
	if timestamp == "" {
		timestamp = time.Now().Format("2006-01-02T15:04:05.000Z07:00")
	} else {
		// Convert RFC3339 to more readable format
		if t, err := time.Parse(time.RFC3339, timestamp); err == nil {
			timestamp = t.Format("2006-01-02T15:04:05.000Z07:00")
		}
	}

	// Create prefix if requested
	var prefix string
	if options.ShowPrefix {
		var prefixParts []string

		if resp.ServiceName != "" {
			prefixParts = append(prefixParts, resp.ServiceName)
		}

		if resp.InstanceName != "" {
			prefixParts = append(prefixParts, resp.InstanceName)
		}

		prefix = nameColor.Sprint(strings.Join(prefixParts, "/")) + " "
	}

	// Output based on format
	switch options.OutputFormat {
	case OutputFormatJSON:
		// Output as JSON
		jsonEntry := map[string]string{
			"timestamp": resp.Timestamp,
			"service":   resp.ServiceName,
			"instance":  resp.InstanceId,
			"content":   content,
			"level":     logLevel,
			"stream":    resp.Stream,
		}

		jsonBytes, err := json.Marshal(jsonEntry)
		if err != nil {
			return err
		}
		fmt.Println(string(jsonBytes))

	case OutputFormatRaw:
		// Output just the content without formatting
		fmt.Println(content)

	case OutputFormatText:
		fallthrough
	default:
		// Format with colors based on log level
		var formattedContent string
		if logLevel == "error" {
			formattedContent = errorColor.Sprint(content)
		} else {
			formattedContent = infoColor.Sprint(content)
		}

		// Normalize line breaks in the content
		formattedContent = strings.TrimSpace(formattedContent)

		// Check if timestamps should be shown
		if options.ShowTimestamps {
			fmt.Printf("%s | %s%s\n",
				timeColor.Sprint(timestamp),
				prefix,
				formattedContent)
		} else {
			fmt.Printf("%s%s\n",
				prefix,
				formattedContent)
		}
	}

	return nil
}
