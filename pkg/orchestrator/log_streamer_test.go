package orchestrator

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
)

// mockReader implements io.ReadCloser for testing
type mockReader struct {
	data   string
	offset int
	closed bool
}

func newMockReader(data string) *mockReader {
	return &mockReader{
		data:   data,
		offset: 0,
		closed: false,
	}
}

func (m *mockReader) Read(p []byte) (n int, err error) {
	if m.closed {
		return 0, io.ErrClosedPipe
	}

	if m.offset >= len(m.data) {
		return 0, io.EOF
	}

	n = copy(p, m.data[m.offset:])
	m.offset += n
	return n, nil
}

func (m *mockReader) Close() error {
	m.closed = true
	return nil
}

// TestMultiLogStreamer_NoMetadata tests log streaming without metadata prefixes
func TestMultiLogStreamer_NoMetadata(t *testing.T) {
	// Create mock readers with test data
	reader1 := newMockReader("log line 1 from instance 1\nlog line 2 from instance 1\n")
	reader2 := newMockReader("log line 1 from instance 2\nlog line 2 from instance 2\n")

	// Create MultiLogStreamer with metadata disabled
	streamer := &MultiLogStreamer{
		readers:  []io.ReadCloser{reader1, reader2},
		buffer:   bytes.NewBuffer(nil),
		done:     make(chan struct{}),
		metadata: false,
	}

	// Start a goroutine for each reader
	for i, reader := range streamer.readers {
		streamer.wg.Add(1)
		instanceInfo := InstanceLogInfo{
			InstanceID:   fmt.Sprintf("inst-%d", i+1),
			InstanceName: fmt.Sprintf("instance-%d", i+1),
			Reader:       reader,
		}
		go streamer.collectLogs(reader, instanceInfo)
	}

	// Start a goroutine to close the streamer when all collectors are done
	go func() {
		streamer.wg.Wait()
		close(streamer.done)
	}()

	defer streamer.Close()

	// Read all data from the streamer
	buf := new(bytes.Buffer)
	_, err := io.Copy(buf, streamer)
	if err != nil {
		t.Fatalf("Failed to read from streamer: %v", err)
	}

	// Verify the output contains all log lines without metadata
	output := buf.String()
	expected := []string{
		"log line 1 from instance 1",
		"log line 2 from instance 1",
		"log line 1 from instance 2",
		"log line 2 from instance 2",
	}

	for _, line := range expected {
		if !strings.Contains(output, line) {
			t.Errorf("Expected output to contain '%s', but it didn't.\nOutput: %s", line, output)
		}
	}
}

// TestMultiLogStreamer_WithMetadata tests log streaming with metadata prefixes
func TestMultiLogStreamer_WithMetadata(t *testing.T) {
	// Create mock readers with test data
	reader1 := newMockReader("log line 1 from instance 1\nlog line 2 from instance 1\n")
	reader2 := newMockReader("log line 1 from instance 2\nlog line 2 from instance 2\n")

	// Create MultiLogStreamer with metadata enabled
	streamer := &MultiLogStreamer{
		readers:  []io.ReadCloser{reader1, reader2},
		buffer:   bytes.NewBuffer(nil),
		done:     make(chan struct{}),
		metadata: true,
	}

	// Start a goroutine for each reader
	for i, reader := range streamer.readers {
		streamer.wg.Add(1)
		instanceInfo := InstanceLogInfo{
			InstanceID:   fmt.Sprintf("inst-%d", i+1),
			InstanceName: fmt.Sprintf("instance-%d", i+1),
			Reader:       reader,
		}
		go streamer.collectLogs(reader, instanceInfo)
	}

	// Start a goroutine to close the streamer when all collectors are done
	go func() {
		streamer.wg.Wait()
		close(streamer.done)
	}()

	defer streamer.Close()

	// Read all data from the streamer
	buf := new(bytes.Buffer)
	_, err := io.Copy(buf, streamer)
	if err != nil {
		t.Fatalf("Failed to read from streamer: %v", err)
	}

	// Verify the output contains all log lines with the correct metadata format
	output := buf.String()
	outputLines := strings.Split(output, "\n")

	// Skip the last empty line after the final newline
	if len(outputLines) > 0 && outputLines[len(outputLines)-1] == "" {
		outputLines = outputLines[:len(outputLines)-1]
	}

	// Check each line has the correct format
	for _, line := range outputLines {
		// Verify line format: [instance-id instance-name timestamp] content
		if len(line) < 10 {
			t.Errorf("Line too short: %s", line)
			continue
		}

		// Check for opening bracket
		if !strings.HasPrefix(line, "[") {
			t.Errorf("Line doesn't start with '[': %s", line)
			continue
		}

		// Check for closing bracket and space before content
		closeBracketPos := strings.Index(line, "] ")
		if closeBracketPos == -1 {
			t.Errorf("Line doesn't contain '] ' separator: %s", line)
			continue
		}

		// Check metadata contains instance ID and name
		metadata := line[1:closeBracketPos]
		if !strings.Contains(metadata, "inst-") || !strings.Contains(metadata, "instance-") {
			t.Errorf("Metadata doesn't contain instance information: %s", metadata)
		}

		// Check for content after metadata
		content := line[closeBracketPos+2:]
		if !strings.Contains(content, "log line") {
			t.Errorf("Content doesn't contain expected log text: %s", content)
		}
	}
}

// TestMultiLogStreamer_CloseReaders tests that closing the streamer closes all readers
func TestMultiLogStreamer_CloseReaders(t *testing.T) {
	// Create mock readers with test data
	reader1 := newMockReader("log line 1\n")
	reader2 := newMockReader("log line 2\n")

	// Create MultiLogStreamer
	streamer := &MultiLogStreamer{
		readers:  []io.ReadCloser{reader1, reader2},
		buffer:   bytes.NewBuffer(nil),
		done:     make(chan struct{}),
		metadata: false,
	}

	// Start a goroutine for each reader
	for i, reader := range streamer.readers {
		streamer.wg.Add(1)
		instanceInfo := InstanceLogInfo{
			InstanceID:   fmt.Sprintf("inst-%d", i+1),
			InstanceName: fmt.Sprintf("instance-%d", i+1),
			Reader:       reader,
		}
		go streamer.collectLogs(reader, instanceInfo)
	}

	// Start a goroutine to close the streamer when all collectors are done
	go func() {
		streamer.wg.Wait()
		close(streamer.done)
	}()

	// Read some data to ensure the collectors are running
	buf := make([]byte, 100)
	_, _ = streamer.Read(buf)

	// Close the streamer
	err := streamer.Close()
	if err != nil {
		t.Fatalf("Failed to close streamer: %v", err)
	}

	// Verify the underlying readers were closed
	if !reader1.closed {
		t.Error("Reader 1 was not closed")
	}

	if !reader2.closed {
		t.Error("Reader 2 was not closed")
	}

	// Verify reading after close returns EOF
	_, err = streamer.Read(buf)
	if err != io.EOF {
		t.Errorf("Expected EOF after close, got: %v", err)
	}
}

// TestMultiLogStreamer_PartialLines tests handling of partial log lines
func TestMultiLogStreamer_PartialLines(t *testing.T) {
	// Create a reader that sends data in chunks that don't align with newlines
	reader1 := newMockReader("line 1 part 1")
	reader2 := newMockReader("line 1 part 2\nline 2\n")

	// Create MultiLogStreamer with metadata enabled
	streamer := &MultiLogStreamer{
		readers:  []io.ReadCloser{reader1, reader2},
		buffer:   bytes.NewBuffer(nil),
		done:     make(chan struct{}),
		metadata: true,
	}

	// Start a goroutine for each reader
	for i, reader := range streamer.readers {
		streamer.wg.Add(1)
		instanceInfo := InstanceLogInfo{
			InstanceID:   fmt.Sprintf("inst-%d", i+1),
			InstanceName: fmt.Sprintf("instance-%d", i+1),
			Reader:       reader,
		}
		go streamer.collectLogs(reader, instanceInfo)
	}

	// Start a goroutine to close the streamer when all collectors are done
	go func() {
		streamer.wg.Wait()
		close(streamer.done)
	}()

	defer streamer.Close()

	// Read all data
	buf := new(bytes.Buffer)
	_, err := io.Copy(buf, streamer)
	if err != nil {
		t.Fatalf("Failed to read from streamer: %v", err)
	}

	// Check output contains proper prefixed lines
	output := buf.String()

	// Verify metadata formatting for complete lines
	if !strings.Contains(output, "[inst-") && !strings.Contains(output, "instance-") {
		t.Errorf("Output doesn't contain expected metadata prefixes:\n%s", output)
	}

	// Verify content
	if !strings.Contains(output, "line 1 part") || !strings.Contains(output, "line 2") {
		t.Errorf("Output doesn't contain expected content:\n%s", output)
	}
}
