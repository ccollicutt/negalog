package parser

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
)

// FileSource implements LogSource for reading from log files.
type FileSource struct {
	files     []string
	extractor *TimestampExtractor

	currentFile    *os.File
	currentScanner *bufio.Scanner
	currentSource  string
	currentLine    int
	fileIndex      int
}

// NewFileSource creates a LogSource that reads from the given files.
// The timestamp pattern and layout are used to extract timestamps from each line.
func NewFileSource(files []string, pattern *regexp.Regexp, layout string) *FileSource {
	return &FileSource{
		files:     files,
		extractor: NewTimestampExtractor(pattern, layout),
		fileIndex: -1,
	}
}

// Next returns the next parsed log line.
// Skips lines that don't match the timestamp pattern.
// Returns io.EOF when all files have been exhausted.
func (s *FileSource) Next(ctx context.Context) (*ParsedLine, error) {
	for {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Ensure we have a file open
		if s.currentScanner == nil {
			if err := s.openNextFile(); err != nil {
				return nil, err
			}
		}

		// Try to read the next line
		if s.currentScanner.Scan() {
			s.currentLine++
			line := s.currentScanner.Text()

			// Try to extract timestamp
			ts, err := s.extractor.Extract(line)
			if err != nil {
				// Skip lines without valid timestamps
				continue
			}

			return &ParsedLine{
				Raw:       line,
				Timestamp: ts,
				Source:    s.currentSource,
				LineNum:   s.currentLine,
			}, nil
		}

		// Check for scanner error
		if err := s.currentScanner.Err(); err != nil {
			return nil, fmt.Errorf("reading %s: %w", s.currentSource, err)
		}

		// Current file exhausted, try next
		if err := s.closeCurrentFile(); err != nil {
			return nil, err
		}
		s.currentScanner = nil
	}
}

// Close releases resources.
func (s *FileSource) Close() error {
	return s.closeCurrentFile()
}

func (s *FileSource) openNextFile() error {
	s.fileIndex++
	if s.fileIndex >= len(s.files) {
		return io.EOF
	}

	path := s.files[s.fileIndex]
	f, err := os.Open(path) // #nosec G304 -- user-provided paths are expected
	if err != nil {
		return fmt.Errorf("opening log file %s: %w", path, err)
	}

	s.currentFile = f
	s.currentScanner = bufio.NewScanner(f)
	s.currentScanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1MB max line size
	s.currentSource = path
	s.currentLine = 0

	return nil
}

func (s *FileSource) closeCurrentFile() error {
	if s.currentFile != nil {
		err := s.currentFile.Close()
		s.currentFile = nil
		s.currentScanner = nil
		return err
	}
	return nil
}
