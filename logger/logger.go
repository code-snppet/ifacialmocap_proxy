package logger

import (
	"sync"
	"time"
)

type Entry struct {
	Time    time.Time
	Level   string
	Message string
}

type Logger struct {
	mu         sync.RWMutex
	entries    []Entry
	notify     chan struct{}
	maxEntries int
}

func NewLogger(maxEntries int) *Logger {
	return &Logger{
		entries:    make([]Entry, 0),
		notify:     make(chan struct{}, 1),
		maxEntries: maxEntries,
	}
}

func (l *Logger) Log(level string, message string) {
	l.mu.Lock()
	l.entries = append(l.entries, Entry{
		Time:    time.Now(),
		Level:   level,
		Message: message,
	})
	if l.maxEntries > 0 && len(l.entries) > l.maxEntries {
		l.entries = l.entries[len(l.entries)-l.maxEntries:]
	}
	l.mu.Unlock()
	l.signal()
}

func (l *Logger) Write(p []byte) (n int, err error) {
	l.Log("INFO", string(p))
	return len(p), nil
}

func (l *Logger) Debug(message string) {
	l.Log("DEBUG", message)
}

func (l *Logger) Info(message string) {
	l.Log("INFO", message)
}

func (l *Logger) Warning(message string) {
	l.Log("WARNING", message)
}

func (l *Logger) Error(message string) {
	l.Log("ERROR", message)
}

func (l *Logger) Entries() []Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]Entry, len(l.entries))
	copy(out, l.entries)
	return out
}

func (l *Logger) Clear() {
	l.mu.Lock()
	l.entries = make([]Entry, 0)
	l.mu.Unlock()
	l.signal()
}

func (l *Logger) signal() {
	select {
	case l.notify <- struct{}{}:
	default:
	}
}

func (l *Logger) NotifyChan() <-chan struct{} {
	return l.notify
}
