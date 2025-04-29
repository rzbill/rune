package orchestrator

import (
	"io"
)

// MultiReadCloser combines multiple io.ReadCloser into one
type MultiReadCloser struct {
	readers []io.ReadCloser
	current int
}

// NewMultiReadCloser creates a new MultiReadCloser
func NewMultiReadCloser(readers ...io.ReadCloser) *MultiReadCloser {
	return &MultiReadCloser{
		readers: readers,
		current: 0,
	}
}

// Read reads from the current reader and advances to the next one when EOF is reached
func (m *MultiReadCloser) Read(p []byte) (n int, err error) {
	if m.current >= len(m.readers) {
		return 0, io.EOF
	}

	n, err = m.readers[m.current].Read(p)
	if err == io.EOF {
		m.current++
		if m.current < len(m.readers) {
			return m.Read(p)
		}
	}

	return n, err
}

// Close closes all readers
func (m *MultiReadCloser) Close() error {
	for _, r := range m.readers {
		if err := r.Close(); err != nil {
			return err
		}
	}

	return nil
}
