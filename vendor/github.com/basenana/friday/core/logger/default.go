package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

type defaultLogger struct {
	name string
	w    io.Writer
}

var (
	defaultLoggerInstance Logger
	defaultLoggerMu       sync.Mutex
)

func New(name string) Logger {
	return &defaultLogger{name: name, w: os.Stdout}
}

func Default() Logger {
	defaultLoggerMu.Lock()
	defer defaultLoggerMu.Unlock()
	if defaultLoggerInstance == nil {
		defaultLoggerInstance = &defaultLogger{name: "default", w: os.Stdout}
	}
	return defaultLoggerInstance
}

func SetDefault(l Logger) {
	defaultLoggerMu.Lock()
	defer defaultLoggerMu.Unlock()
	defaultLoggerInstance = l
}

func (l *defaultLogger) With(keysAndValues ...interface{}) Logger {
	newLogger := &defaultLogger{
		name: l.name,
		w:    l.w,
	}
	return newLogger
}

func (l *defaultLogger) Info(args ...interface{}) {
	fmt.Fprint(l.w, args...)
	fmt.Fprintln(l.w)
}

func (l *defaultLogger) Warn(args ...interface{}) {
	fmt.Fprint(l.w, args...)
	fmt.Fprintln(l.w)
}

func (l *defaultLogger) Error(args ...interface{}) {
	fmt.Fprint(l.w, args...)
	fmt.Fprintln(l.w)
}

func (l *defaultLogger) Infof(template string, args ...interface{}) {
	fmt.Fprintf(l.w, template+"\n", args...)
}

func (l *defaultLogger) Warnf(template string, args ...interface{}) {
	fmt.Fprintf(l.w, template+"\n", args...)
}

func (l *defaultLogger) Errorf(template string, args ...interface{}) {
	fmt.Fprintf(l.w, template+"\n", args...)
}

func (l *defaultLogger) Infow(msg string, keysAndValues ...interface{}) {
	l.logWithKeys("INFO", msg, keysAndValues)
}

func (l *defaultLogger) Warnw(msg string, keysAndValues ...interface{}) {
	l.logWithKeys("WARN", msg, keysAndValues)
}

func (l *defaultLogger) Errorw(msg string, keysAndValues ...interface{}) {
	l.logWithKeys("ERROR", msg, keysAndValues)
}

func (l *defaultLogger) logWithKeys(level, msg string, keysAndValues []interface{}) {
	var kvStr string
	if len(keysAndValues) > 0 {
		kvStr = " [" + formatKeyValues(keysAndValues) + "]"
	}
	if l.name != "" {
		fmt.Fprintf(l.w, "%s: [%s] %s%s\n", level, l.name, msg, kvStr)
	} else {
		fmt.Fprintf(l.w, "%s: %s%s\n", level, msg, kvStr)
	}
}

func formatKeyValues(kv []interface{}) string {
	result := ""
	for i := 0; i+1 < len(kv); i += 2 {
		if i > 0 {
			result += ", "
		}
		key := fmt.Sprintf("%v", kv[i])
		value := fmt.Sprintf("%v", kv[i+1])
		result += key + "=" + value
	}
	return result
}

func FirstLine(s string) string {
	if i := strings.Index(s, "\n"); i != -1 {
		return s[:i]
	}
	return s
}
