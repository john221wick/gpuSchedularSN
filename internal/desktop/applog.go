package desktop

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// AppLogger captures application logs in a ring buffer for the UI.
var appLogger = &AppLog{
	maxLines: 500,
}

func init() {
	// Tee stdout to both terminal and our buffer
	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				// Write to original stdout (terminal)
				origStdout.Write(buf[:n])
				// Capture in log buffer
				appLogger.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
	}()
}

type logEntry struct {
	Time    string `json:"time"`
	Message string `json:"message"`
}

type AppLog struct {
	mu       sync.RWMutex
	entries  []logEntry
	maxLines int
	buf      []byte // partial line buffer
}

func (l *AppLog) Write(p []byte) (n int, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.buf = append(l.buf, p...)

	// Split by newlines
	for {
		idx := strings.IndexByte(string(l.buf), '\n')
		if idx < 0 {
			break
		}
		line := strings.TrimRight(string(l.buf[:idx]), "\r")
		l.buf = l.buf[idx+1:]

		if line == "" {
			continue
		}

		l.entries = append(l.entries, logEntry{
			Time:    time.Now().Format("15:04:05"),
			Message: line,
		})

		// Trim if over max
		if len(l.entries) > l.maxLines {
			l.entries = l.entries[len(l.entries)-l.maxLines:]
		}
	}

	return len(p), nil
}

func (l *AppLog) GetEntries(offset int) []logEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if offset >= len(l.entries) {
		return nil
	}
	if offset < 0 {
		offset = 0
	}
	result := make([]logEntry, len(l.entries)-offset)
	copy(result, l.entries[offset:])
	return result
}

func (l *AppLog) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.entries)
}

// LogPrintf writes to both the app log and stdout.
func LogPrintf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

// Ensure AppLog implements io.Writer
var _ io.Writer = (*AppLog)(nil)
