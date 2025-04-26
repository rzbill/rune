package docker

import (
	"bytes"
	"encoding/binary"
	"io"
)

// dockerLogReader is an io.ReadCloser that demultiplexes the Docker log format
// which interleaves stdout and stderr with headers.
type dockerLogReader struct {
	reader io.ReadCloser
	buffer *bytes.Buffer
	header [8]byte // Docker log header is 8 bytes
	remain int     // Remaining bytes in the current frame
}

// newLogReader creates a new log reader that demultiplexes Docker logs.
func newLogReader(reader io.ReadCloser) io.ReadCloser {
	return &dockerLogReader{
		reader: reader,
		buffer: bytes.NewBuffer(nil),
		remain: 0,
	}
}

// Read implements io.Reader for the log reader.
// This function handles the Docker log format, which consists of:
// - 8-byte header: [stream type byte][0 byte][0 byte][0 byte][frame size as uint32]
// - frame content: [frame size bytes]
// This repeats for each frame. We need to strip the headers and return just the content.
func (r *dockerLogReader) Read(p []byte) (int, error) {
	// Return any data we already have in the buffer
	if r.buffer.Len() > 0 {
		return r.buffer.Read(p)
	}

	// If we don't have any data remaining in the current frame, read a new frame
	if r.remain == 0 {
		// Read the 8-byte header
		if _, err := io.ReadFull(r.reader, r.header[:]); err != nil {
			if err == io.EOF {
				return 0, io.EOF
			}
			return 0, err
		}

		// Extract the frame size from bytes 4-7 (uint32, big endian)
		r.remain = int(binary.BigEndian.Uint32(r.header[4:]))
	}

	// Read up to the remaining bytes in the frame
	toRead := len(p)
	if toRead > r.remain {
		toRead = r.remain
	}

	// Read the content
	n, err := io.ReadFull(r.reader, p[:toRead])
	r.remain -= n

	if err != nil && err != io.ErrUnexpectedEOF {
		return n, err
	}

	return n, nil
}

// Close implements io.Closer.
func (r *dockerLogReader) Close() error {
	return r.reader.Close()
}
