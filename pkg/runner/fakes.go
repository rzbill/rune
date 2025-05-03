package runner

import (
	"bytes"
	"fmt"
	"io"
	"sync"
)

// FakeExecStream implements ExecStream for testing
type FakeExecStream struct {
	StdoutContent []byte
	StdoutPos     int
	StderrContent []byte
	StderrReader  io.Reader
	ExitCodeVal   int
	SignalsSent   []string
	InputCapture  []byte
	Closed        bool
	mu            sync.Mutex
}

// NewFakeExecStream creates a new fake exec stream with predefined content
func NewFakeExecStream(stdout, stderr []byte, exitCode int) *FakeExecStream {
	return &FakeExecStream{
		StdoutContent: stdout,
		StderrContent: stderr,
		ExitCodeVal:   exitCode,
		StderrReader:  bytes.NewReader(stderr),
		SignalsSent:   make([]string, 0),
		InputCapture:  make([]byte, 0),
		Closed:        false,
	}
}

// Write captures input that would be sent to the exec process
func (s *FakeExecStream) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Closed {
		return 0, io.ErrClosedPipe
	}

	// Capture the input
	s.InputCapture = append(s.InputCapture, p...)
	return len(p), nil
}

// Read returns predefined output content in chunks
func (s *FakeExecStream) Read(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Closed {
		return 0, io.ErrClosedPipe
	}

	// If we've read everything, return EOF
	if s.StdoutPos >= len(s.StdoutContent) {
		return 0, io.EOF
	}

	// Calculate how much to read
	remaining := len(s.StdoutContent) - s.StdoutPos
	toRead := len(p)
	if toRead > remaining {
		toRead = remaining
	}

	// Copy the data
	copy(p, s.StdoutContent[s.StdoutPos:s.StdoutPos+toRead])
	s.StdoutPos += toRead

	return toRead, nil
}

// Stderr returns an io.Reader for the stderr stream
func (s *FakeExecStream) Stderr() io.Reader {
	return s.StderrReader
}

// ResizeTerminal records terminal resize events
func (s *FakeExecStream) ResizeTerminal(width, height uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Closed {
		return fmt.Errorf("exec session closed")
	}

	return nil
}

// Signal records signals sent to the process
func (s *FakeExecStream) Signal(sigName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Closed {
		return fmt.Errorf("exec session closed")
	}

	s.SignalsSent = append(s.SignalsSent, sigName)
	return nil
}

// ExitCode returns the predefined exit code
func (s *FakeExecStream) ExitCode() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.ExitCodeVal, nil
}

// Close marks the stream as closed
func (s *FakeExecStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Closed = true
	return nil
}
