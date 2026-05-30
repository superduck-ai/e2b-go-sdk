package template

import (
	"fmt"
	"regexp"
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

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;:]*[A-Za-z]`)

func stripAnsi(text string) string {
	return ansiRegex.ReplaceAllString(text, "")
}

func (e *LogEntry) String() string {
	return fmt.Sprintf("[%s] [%s] %s", e.Timestamp.Format(time.RFC3339), e.Level, stripAnsi(e.Message))
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

var defaultBuildLoggerLevelOrder = map[LogEntryLevel]int{
	LogLevelDebug: 0,
	LogLevelInfo:  1,
	LogLevelWarn:  2,
	LogLevelError: 3,
}

func DefaultBuildLogger() BuildLogger {
	return func(entry *LogEntry) {
		if defaultBuildLoggerLevelOrder[entry.Level] < defaultBuildLoggerLevelOrder[LogLevelInfo] {
			return
		}
		msg := strings.TrimRight(stripAnsi(entry.Message), "\n")
		fmt.Printf("[%s] %s\n", entry.Level, msg)
	}
}
