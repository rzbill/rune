package runner

import "io"

// Reader is an interface that represents a readable stream.
// It extends the standard io.Reader interface.
type Reader interface {
	io.Reader
}
