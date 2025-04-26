package log

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ConsoleOutput writes log entries to the console (stdout/stderr).
type ConsoleOutput struct {
	mu            sync.Mutex
	useStderr     bool      // Use stderr instead of stdout
	errorToStderr bool      // Send error and fatal logs to stderr
	writer        io.Writer // Custom writer (optional)
	errorWriter   io.Writer // Custom error writer (optional)
}

// Write writes the log entry to the console.
func (o *ConsoleOutput) Write(entry *Entry, formattedEntry []byte) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Determine the appropriate writer
	var writer io.Writer
	if o.writer != nil {
		writer = o.writer
	} else if o.useStderr {
		writer = os.Stderr
	} else {
		writer = os.Stdout
	}

	// Use error writer for error/fatal levels if enabled
	if (entry.Level == ErrorLevel || entry.Level == FatalLevel) && o.errorToStderr {
		if o.errorWriter != nil {
			writer = o.errorWriter
		} else {
			writer = os.Stderr
		}
	}

	_, err := writer.Write(formattedEntry)
	return err
}

// Close implements the Output interface but does nothing for console output.
func (o *ConsoleOutput) Close() error {
	return nil
}

// ConsoleOutputOption is a function that configures a ConsoleOutput.
type ConsoleOutputOption func(*ConsoleOutput)

// WithStderr configures the ConsoleOutput to use stderr.
func WithStderr() ConsoleOutputOption {
	return func(o *ConsoleOutput) {
		o.useStderr = true
	}
}

// WithErrorToStderr configures the ConsoleOutput to send error and fatal logs to stderr.
func WithErrorToStderr() ConsoleOutputOption {
	return func(o *ConsoleOutput) {
		o.errorToStderr = true
	}
}

// WithCustomWriter configures the ConsoleOutput to use a custom writer.
func WithCustomWriter(writer io.Writer) ConsoleOutputOption {
	return func(o *ConsoleOutput) {
		o.writer = writer
	}
}

// WithCustomErrorWriter configures the ConsoleOutput to use a custom error writer.
func WithCustomErrorWriter(writer io.Writer) ConsoleOutputOption {
	return func(o *ConsoleOutput) {
		o.errorWriter = writer
	}
}

// NewConsoleOutput creates a new ConsoleOutput with the given options.
func NewConsoleOutput(options ...ConsoleOutputOption) *ConsoleOutput {
	o := &ConsoleOutput{
		errorToStderr: true, // Default to sending errors to stderr
	}

	for _, option := range options {
		option(o)
	}

	return o
}

// FileOutput writes log entries to a file.
type FileOutput struct {
	mu           sync.Mutex
	file         *os.File
	filename     string
	maxSize      int64         // Maximum file size in bytes
	maxAge       time.Duration // Maximum file age
	maxBackups   int           // Maximum number of backups
	backupFormat string        // Format for backup filenames
	currentSize  int64         // Current file size
	openTime     time.Time     // When the file was opened
}

// Write writes the log entry to the file.
func (o *FileOutput) Write(entry *Entry, formattedEntry []byte) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Ensure file is open
	if o.file == nil {
		if err := o.openFile(); err != nil {
			return err
		}
	}

	// Check if rotation is needed
	if o.shouldRotate(int64(len(formattedEntry))) {
		if err := o.rotate(); err != nil {
			return err
		}
	}

	// Write to file
	n, err := o.file.Write(formattedEntry)
	if err != nil {
		return err
	}

	// Update current size
	o.currentSize += int64(n)
	return nil
}

// Close closes the file.
func (o *FileOutput) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.file != nil {
		err := o.file.Close()
		o.file = nil
		return err
	}
	return nil
}

// shouldRotate checks if the file should be rotated.
func (o *FileOutput) shouldRotate(size int64) bool {
	// Check size limit
	if o.maxSize > 0 && o.currentSize+size > o.maxSize {
		return true
	}

	// Check age limit
	if o.maxAge > 0 && time.Since(o.openTime) > o.maxAge {
		return true
	}

	return false
}

// openFile opens the log file.
func (o *FileOutput) openFile() error {
	// Create directory if needed
	dir := filepath.Dir(o.filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Open file for appending
	file, err := os.OpenFile(o.filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	// Get file info for size
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return err
	}

	o.file = file
	o.currentSize = info.Size()
	o.openTime = time.Now()
	return nil
}

// rotate rotates the log file.
func (o *FileOutput) rotate() error {
	// Close current file
	if o.file != nil {
		if err := o.file.Close(); err != nil {
			return err
		}
		o.file = nil
	}

	// Generate backup name
	backupName := o.backupFilename()

	// Rename current file to backup
	if err := os.Rename(o.filename, backupName); err != nil && !os.IsNotExist(err) {
		return err
	}

	// Clean up old backups
	if o.maxBackups > 0 {
		if err := o.cleanupOldBackups(); err != nil {
			return err
		}
	}

	// Open new file
	return o.openFile()
}

// backupFilename generates a filename for a backup.
func (o *FileOutput) backupFilename() string {
	format := o.backupFormat
	if format == "" {
		format = "2006-01-02T15-04-05"
	}
	timestamp := time.Now().Format(format)
	return fmt.Sprintf("%s.%s", o.filename, timestamp)
}

// cleanupOldBackups removes old backup files.
func (o *FileOutput) cleanupOldBackups() error {
	dir := filepath.Dir(o.filename)
	base := filepath.Base(o.filename)

	// List all backup files
	files, err := filepath.Glob(filepath.Join(dir, base+".*"))
	if err != nil {
		return err
	}

	// If we have fewer backups than the limit, no cleanup needed
	if len(files) <= o.maxBackups {
		return nil
	}

	// Get file info for sorting
	type backupFile struct {
		path    string
		modTime time.Time
	}
	backups := make([]backupFile, 0, len(files))
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}
		backups = append(backups, backupFile{path: file, modTime: info.ModTime()})
	}

	// Sort by modification time (newest first)
	for i := 0; i < len(backups)-1; i++ {
		for j := i + 1; j < len(backups); j++ {
			if backups[i].modTime.Before(backups[j].modTime) {
				backups[i], backups[j] = backups[j], backups[i]
			}
		}
	}

	// Remove excess backups
	for i := o.maxBackups; i < len(backups); i++ {
		if err := os.Remove(backups[i].path); err != nil {
			return err
		}
	}

	return nil
}

// FileOutputOption is a function that configures a FileOutput.
type FileOutputOption func(*FileOutput)

// WithMaxSize sets the maximum file size.
func WithMaxSize(maxBytes int64) FileOutputOption {
	return func(o *FileOutput) {
		o.maxSize = maxBytes
	}
}

// WithMaxAge sets the maximum file age.
func WithMaxAge(maxAge time.Duration) FileOutputOption {
	return func(o *FileOutput) {
		o.maxAge = maxAge
	}
}

// WithMaxBackups sets the maximum number of backup files.
func WithMaxBackups(maxBackups int) FileOutputOption {
	return func(o *FileOutput) {
		o.maxBackups = maxBackups
	}
}

// WithBackupFormat sets the format for backup filenames.
func WithBackupFormat(format string) FileOutputOption {
	return func(o *FileOutput) {
		o.backupFormat = format
	}
}

// NewFileOutput creates a new FileOutput with the given options.
func NewFileOutput(filename string, options ...FileOutputOption) *FileOutput {
	o := &FileOutput{
		filename:     filename,
		maxSize:      10 * 1024 * 1024, // 10MB default
		maxBackups:   5,                // 5 backups default
		backupFormat: "2006-01-02T15-04-05",
	}

	for _, option := range options {
		option(o)
	}

	return o
}

// NullOutput discards all log entries.
type NullOutput struct{}

// Write implements the Output interface but does nothing.
func (o *NullOutput) Write(entry *Entry, formattedEntry []byte) error {
	return nil
}

// Close implements the Output interface but does nothing.
func (o *NullOutput) Close() error {
	return nil
}

// NewNullOutput creates a new NullOutput.
func NewNullOutput() *NullOutput {
	return &NullOutput{}
}
