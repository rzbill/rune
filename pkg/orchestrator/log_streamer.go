package orchestrator

import (
	"bytes"
	"fmt"
	"io"
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
	InstanceID string
	Reader     io.ReadCloser
}

// NewMultiLogStreamer creates a new MultiLogStreamer that combines log streams from multiple instances
func NewMultiLogStreamer(instances []InstanceLogInfo, includeMetadata bool) *MultiLogStreamer {
	m := &MultiLogStreamer{
		readers:  make([]io.ReadCloser, len(instances)),
		buffer:   bytes.NewBuffer(nil),
		done:     make(chan struct{}),
		metadata: includeMetadata,
	}

	// Extract readers for internal tracking and store instance IDs
	instanceIDs := make([]string, len(instances))
	for i, info := range instances {
		m.readers[i] = info.Reader
		instanceIDs[i] = info.InstanceID
	}

	// Start a goroutine for each reader
	for i, reader := range m.readers {
		m.wg.Add(1)
		instanceID := instanceIDs[i]
		go m.collectLogs(reader, instanceID)
	}

	// Start a goroutine to close the streamer when all collectors are done
	go func() {
		m.wg.Wait()
		close(m.done)
	}()

	return m
}

// collectLogs reads from a reader and writes to the buffer with instance metadata
func (m *MultiLogStreamer) collectLogs(reader io.ReadCloser, instanceID string) {
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
						timestamp := time.Now().Format("2006-01-02T15:04:05.000Z")
						prefix := fmt.Sprintf("[%s %s] ", instanceID, timestamp)
						m.buffer.WriteString(prefix)
						m.buffer.Write(lineBuffer)
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
				fmt.Printf("Error reading from %s logs: %v\n", instanceID, err)
			}

			// If we have any remaining data in the line buffer, write it out
			if m.metadata && len(lineBuffer) > 0 {
				m.bufMu.Lock()
				timestamp := time.Now().Format("2006-01-02T15:04:05.000Z")
				prefix := fmt.Sprintf("[%s %s] ", instanceID, timestamp)
				m.buffer.WriteString(prefix)
				m.buffer.Write(lineBuffer)
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
