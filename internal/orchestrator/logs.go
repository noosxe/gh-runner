package orchestrator

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/pkg/stdcopy"
)

// LogEntry represents a structured log line captured from a runner container.
type LogEntry struct {
	Timestamp string `json:"timestamp"` // RFC3339 format
	Stream    string `json:"stream"`    // "stdout" or "stderr"
	Content   string `json:"content"`   // log line content
}

// LogPath returns the expected location for a runner's compressed JSONL log file:
// DATA_DIR/logs/<runner-id>.log.jsonl.gz (OQ #14, #20).
func LogPath(dataDir, runnerID string) string {
	return filepath.Join(dataDir, "logs", fmt.Sprintf("%s.log.jsonl.gz", runnerID))
}

// CaptureAndCompressLogs parses Docker multiplexed logs from stream, formats them as structured
// JSONL entries with timestamps and stream type, and writes them gzip-compressed to destPath.
func CaptureAndCompressLogs(stream io.Reader, destPath string) (int, error) {
	if stream == nil {
		return 0, fmt.Errorf("stream reader cannot be nil")
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return 0, fmt.Errorf("creating log directory: %w", err)
	}

	var entries []LogEntry

	stdoutCollector := &lineCollector{stream: "stdout", lines: &entries}
	stderrCollector := &lineCollector{stream: "stderr", lines: &entries}

	// Read stream data into buffer to allow fallback if not multiplexed
	data, err := io.ReadAll(stream)
	if err != nil && err != io.EOF {
		return 0, fmt.Errorf("reading container log stream: %w", err)
	}

	// Try demultiplexing using Docker StdCopy
	reader := bytes.NewReader(data)
	_, copyErr := stdcopy.StdCopy(stdoutCollector, stderrCollector, reader)
	stdoutCollector.Flush()
	stderrCollector.Flush()

	if copyErr != nil || len(entries) == 0 {
		// Fallback for plain / non-multiplexed streams (e.g. TTY or raw reader)
		entries = entries[:0]
		scanner := bufio.NewScanner(bytes.NewReader(data))
		for scanner.Scan() {
			text := scanner.Text()
			if strings.TrimSpace(text) != "" {
				entries = append(entries, parseLogLine(text, "stdout"))
			}
		}
	}

	// Stable sort entries by timestamp
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Timestamp < entries[j].Timestamp
	})

	outFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return 0, fmt.Errorf("creating compressed log file %q: %w", destPath, err)
	}
	defer func() { _ = outFile.Close() }()

	gzWriter := gzip.NewWriter(outFile)
	defer func() { _ = gzWriter.Close() }()

	encoder := json.NewEncoder(gzWriter)
	for _, entry := range entries {
		if err := encoder.Encode(entry); err != nil {
			return 0, fmt.Errorf("encoding log entry to JSONL: %w", err)
		}
	}

	if err := gzWriter.Close(); err != nil {
		return 0, fmt.Errorf("closing gzip stream: %w", err)
	}

	return len(entries), nil
}

// ReadGzippedJSONLLogs decompresses and parses a .log.jsonl.gz file into structured LogEntry items.
func ReadGzippedJSONLLogs(srcPath string) ([]LogEntry, error) {
	f, err := os.Open(srcPath)
	if err != nil {
		return nil, fmt.Errorf("opening log file %q: %w", srcPath, err)
	}
	defer func() { _ = f.Close() }()

	gzReader, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("reading gzip stream from %q: %w", srcPath, err)
	}
	defer func() { _ = gzReader.Close() }()

	var entries []LogEntry
	scanner := bufio.NewScanner(gzReader)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var entry LogEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			return nil, fmt.Errorf("parsing JSONL line %q: %w", string(line), err)
		}
		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning JSONL lines: %w", err)
	}

	return entries, nil
}

type lineCollector struct {
	stream string
	lines  *[]LogEntry
	buf    bytes.Buffer
}

func (l *lineCollector) Write(p []byte) (int, error) {
	l.buf.Write(p)
	for {
		line, err := l.buf.ReadBytes('\n')
		if err != nil {
			// Incomplete line remains in buffer for next write
			l.buf.Write(line)
			break
		}
		clean := strings.TrimRight(string(line), "\r\n")
		if clean != "" {
			*l.lines = append(*l.lines, parseLogLine(clean, l.stream))
		}
	}
	return len(p), nil
}

func (l *lineCollector) Flush() {
	if l.buf.Len() > 0 {
		clean := strings.TrimRight(l.buf.String(), "\r\n")
		if clean != "" {
			*l.lines = append(*l.lines, parseLogLine(clean, l.stream))
		}
		l.buf.Reset()
	}
}

func parseLogLine(raw, stream string) LogEntry {
	parts := strings.SplitN(raw, " ", 2)
	if len(parts) == 2 {
		if t, err := time.Parse(time.RFC3339Nano, parts[0]); err == nil {
			return LogEntry{
				Timestamp: t.UTC().Format(time.RFC3339),
				Stream:    stream,
				Content:   parts[1],
			}
		}
		if t, err := time.Parse(time.RFC3339, parts[0]); err == nil {
			return LogEntry{
				Timestamp: t.UTC().Format(time.RFC3339),
				Stream:    stream,
				Content:   parts[1],
			}
		}
	}

	return LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Stream:    stream,
		Content:   raw,
	}
}
