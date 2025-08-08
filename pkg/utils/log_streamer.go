package utils

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// MultiLogStreamer combines log streams from multiple instances into a single stream
type MultiLogStreamer struct {
	readers  []io.ReadCloser
	buffer   *bytes.Buffer
	bufMu    sync.Mutex
	closed   bool
	closeMu  sync.Mutex
	done     chan struct{}
	wg       sync.WaitGroup
	metadata bool
}

// InstanceLogInfo associates an instance ID with its log reader
type InstanceLogInfo struct {
	InstanceID   string
	InstanceName string
	Reader       io.ReadCloser
}

// NewMultiLogStreamer creates a new MultiLogStreamer that combines log streams from multiple instances
func NewMultiLogStreamer(instances []InstanceLogInfo, includeMetadata bool) *MultiLogStreamer {
	m := &MultiLogStreamer{
		readers:  make([]io.ReadCloser, len(instances)),
		buffer:   bytes.NewBuffer(nil),
		done:     make(chan struct{}),
		metadata: includeMetadata,
	}

	// Copy the readers from instances to the readers array
	for i, instance := range instances {
		m.readers[i] = instance.Reader
	}

	// Start a goroutine for each reader
	for i, reader := range m.readers {
		m.wg.Add(1)
		go m.collectLogs(reader, instances[i])
	}

	// Start a goroutine to close the streamer when all collectors are done
	go func() {
		m.wg.Wait()
		close(m.done)
	}()

	return m
}

// collectLogs reads from a reader and writes to the buffer with instance metadata
func (m *MultiLogStreamer) collectLogs(reader io.ReadCloser, instance InstanceLogInfo) {
	defer m.wg.Done()
	defer reader.Close()

	buf := make([]byte, 4096)
	lineBuffer := make([]byte, 0, 4096)

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			m.bufMu.Lock()

			// Process the buffer line by line to add metadata at the beginning of each line
			if m.metadata {
				data := buf[:n]
				for i := 0; i < n; i++ {
					lineBuffer = append(lineBuffer, data[i])

					// If we have a full line or this is the last chunk of data
					if data[i] == '\n' || (err == io.EOF && i == n-1) {
						// Check if line already has our metadata format
						lineStr := string(lineBuffer)
						alreadyHasMetadata := lineHasMetadata(lineStr)

						if alreadyHasMetadata {
							// If already has metadata, write as is
							m.buffer.Write(lineBuffer)
						} else {
							// Add metadata prefix
							prefix := buildLineMetadata(instance)
							m.buffer.WriteString(prefix)
							m.buffer.Write(lineBuffer)
						}

						lineBuffer = lineBuffer[:0] // Clear the line buffer
					}
				}
			} else {
				// If not adding metadata, just write the data as is
				m.buffer.Write(buf[:n])
			}

			m.bufMu.Unlock()
		}

		if err != nil {
			if err != io.EOF {
				// Log error but continue with other readers
				fmt.Printf("Error reading from %s logs: %v\n", instance.InstanceID, err)
			}

			// If we have any remaining data in the line buffer, write it out
			if m.metadata && len(lineBuffer) > 0 {
				m.bufMu.Lock()

				// Check if line already has our metadata format
				lineStr := string(lineBuffer)
				alreadyHasMetadata := lineHasMetadata(lineStr)

				if alreadyHasMetadata {
					// If already has metadata, write as is
					m.buffer.Write(lineBuffer)
				} else {
					// Add metadata prefix
					prefix := buildLineMetadata(instance)
					m.buffer.WriteString(prefix)
					m.buffer.Write(lineBuffer)
				}

				m.bufMu.Unlock()
			}

			break
		}
	}
}

// Read implements the io.Reader interface
func (m *MultiLogStreamer) Read(p []byte) (n int, err error) {
	for {
		// Check if we have data in the buffer
		m.bufMu.Lock()
		if m.buffer.Len() > 0 {
			n, err = m.buffer.Read(p)
			m.bufMu.Unlock()
			return
		}
		m.bufMu.Unlock()

		// If not, check if we're done
		select {
		case <-m.done:
			if m.closed {
				return 0, io.EOF
			}
			// One last check for data before returning EOF
			m.bufMu.Lock()
			n, err = m.buffer.Read(p)
			m.bufMu.Unlock()
			if n > 0 {
				return n, err
			}
			return 0, io.EOF
		default:
			// Wait a bit before checking again
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// Close implements the io.Closer interface
func (m *MultiLogStreamer) Close() error {
	m.closeMu.Lock()
	defer m.closeMu.Unlock()

	if m.closed {
		return nil
	}
	m.closed = true

	// Close all readers
	var firstErr error
	for _, reader := range m.readers {
		if err := reader.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// lineHasMetadata checks if a line already has our metadata format
func lineHasMetadata(lineStr string) bool {
	// Check for our metadata format: @@LOG_META|[uuid|name|timestamp]@@
	if len(lineStr) > 15 && strings.HasPrefix(lineStr, "@@LOG_META|[") {
		closingMarkerPos := strings.Index(lineStr, "]@@")
		if closingMarkerPos > 0 {
			return true
		}
	}
	return false
}

// ExtractLineMetadata extracts metadata and content from a line
// Returns metadata (instanceID, instanceName, timestamp) and the remaining content
func ExtractLineMetadata(lineStr string) (string, string, string, string) {
	if !lineHasMetadata(lineStr) {
		return "", "", "", lineStr
	}

	// Find metadata section
	metadataStart := len("@@LOG_META|[")
	metadataEnd := strings.Index(lineStr, "]@@")

	// Extract and parse metadata
	metadata := lineStr[metadataStart:metadataEnd]
	parts := strings.Split(metadata, "|")

	// Default values
	instanceID, instanceName, timestamp := "", "", ""

	// Extract metadata fields if available
	if len(parts) >= 3 {
		instanceID = parts[0]
		instanceName = parts[1]
		timestamp = parts[2]
	}

	// Extract content (everything after the metadata)
	content := lineStr[metadataEnd+3:] // Skip past "]@@"

	// Remove leading space if present
	if strings.HasPrefix(content, " ") && len(content) > 1 {
		content = content[1:]
	}

	return instanceID, instanceName, timestamp, content
}

// buildLineMetadata creates a metadata prefix for log lines
func buildLineMetadata(instance InstanceLogInfo) string {
	timestamp := time.Now().Format("2006-01-02T15:04:05.000Z")

	// Format: @@LOG_META|[instanceID|instanceName|timestamp]@@
	// This format is distinct enough to avoid confusion with application logs
	prefix := fmt.Sprintf("@@LOG_META|[%s|%s|%s]@@ ",
		instance.InstanceID,
		instance.InstanceName,
		timestamp)

	return prefix
}
