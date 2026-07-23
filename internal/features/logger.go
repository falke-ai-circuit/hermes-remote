package features

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

type Logger struct {
	mu       sync.Mutex
	output   io.Writer
	level    LogLevel
	maxSize  int64
	filePath string
}

func NewLogger(path string, level LogLevel) (*Logger, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	return &Logger{output: f, level: level, maxSize: 10 * 1024 * 1024, filePath: path}, nil
}

func (l *Logger) Log(level LogLevel, component, message string, fields map[string]interface{}) {
	if level < l.level {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	timestamp := time.Now().Format(time.RFC3339)
	levelStr := levelToString(level)
	line := fmt.Sprintf("[%s] %s %s: %s", timestamp, levelStr, component, message)
	if len(fields) > 0 {
		extras := make([]string, 0, len(fields))
		for k, v := range fields {
			extras = append(extras, fmt.Sprintf("%s=%v", k, v))
		}
		line += " " + joinStrings(extras, " ")
	}
	line += "\n"
	io.WriteString(l.output, line)
}

func (l *Logger) Info(component, message string) {
	l.Log(LevelInfo, component, message, nil)
}

func (l *Logger) Warn(component, message string) {
	l.Log(LevelWarn, component, message, nil)
}

func (l *Logger) Error(component, message string, err error) {
	fields := map[string]interface{}{}
	if err != nil {
		fields["error"] = err.Error()
	}
	l.Log(LevelError, component, message, fields)
}

func (l *Logger) Debug(component, message string) {
	l.Log(LevelDebug, component, message, nil)
}

func levelToString(level LogLevel) string {
	switch level {
	case LevelDebug: return "DEBUG"
	case LevelInfo: return "INFO"
	case LevelWarn: return "WARN"
	case LevelError: return "ERROR"
	default: return "UNKNOWN"
	}
}

func joinStrings(items []string, sep string) string {
	result := ""
	for i, item := range items {
		if i > 0 { result += sep }
		result += item
	}
	return result
}

func (l *Logger) Close() error {
	if closer, ok := l.output.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
