package template

import (
	"fmt"
	"strings"
	"time"
)

type LogEntryLevel string

const (
	LogLevelDebug LogEntryLevel = "debug"
	LogLevelInfo  LogEntryLevel = "info"
	LogLevelWarn  LogEntryLevel = "warn"
	LogLevelError LogEntryLevel = "error"
)

type LogEntry struct {
	Timestamp time.Time
	Level     LogEntryLevel
	Message   string
}

func (e *LogEntry) String() string {
	return fmt.Sprintf("[%s] %s: %s", e.Timestamp.Format(time.RFC3339), e.Level, e.Message)
}

type LogEntryStart struct {
	LogEntry
}

func NewLogEntryStart(message string) *LogEntryStart {
	return &LogEntryStart{LogEntry: LogEntry{Timestamp: time.Now(), Level: LogLevelDebug, Message: message}}
}

type LogEntryEnd struct {
	LogEntry
}

func NewLogEntryEnd(message string) *LogEntryEnd {
	return &LogEntryEnd{LogEntry: LogEntry{Timestamp: time.Now(), Level: LogLevelDebug, Message: message}}
}

type BuildLogger func(entry *LogEntry)

func DefaultBuildLogger() BuildLogger {
	return func(entry *LogEntry) {
		msg := strings.TrimRight(entry.Message, "\n")
		fmt.Printf("[%s] %s\n", entry.Level, msg)
	}
}
