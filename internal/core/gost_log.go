package core

import (
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"

	"github.com/go-gost/core/logger"
)

type gostLogSink struct{ emit func(string, string) }

var gostSink atomic.Pointer[gostLogSink]
var logUserInfo = regexp.MustCompile(`(?i)([a-z][a-z0-9+.-]*://)[^\s/]*@`)

// SetGostLogSink forwards warnings/errors to the application without changing
// global log levels or exposing debug/trace authentication/config dumps.
func SetGostLogSink(emit func(string, string)) {
	if emit == nil {
		gostSink.Store(nil)
		return
	}
	gostSink.Store(&gostLogSink{emit})
}

type gostEventLogger struct {
	logger.Logger
	kind string
}

func (l *gostEventLogger) WithFields(fields map[string]any) logger.Logger {
	kind := l.kind
	if v, ok := fields["kind"].(string); ok {
		kind = v
	}
	return &gostEventLogger{Logger: l.Logger.WithFields(fields), kind: kind}
}

func (l *gostEventLogger) emit(level, msg string) {
	sink := gostSink.Load()
	if sink == nil || strings.Contains(msg, "use of closed network connection") {
		return
	}
	msg = logUserInfo.ReplaceAllString(msg, "${1}***@")
	kind := l.kind
	if kind == "" {
		kind = "core"
	}
	sink.emit(level, fmt.Sprintf("GOST [%s]: %s", kind, msg))
}

func (l *gostEventLogger) Warn(args ...any) {
	l.Logger.Warn(args...)
	l.emit("warn", fmt.Sprint(args...))
}
func (l *gostEventLogger) Warnf(format string, args ...any) {
	l.Logger.Warnf(format, args...)
	l.emit("warn", fmt.Sprintf(format, args...))
}
func (l *gostEventLogger) Error(args ...any) {
	l.Logger.Error(args...)
	l.emit("error", fmt.Sprint(args...))
}
func (l *gostEventLogger) Errorf(format string, args ...any) {
	l.Logger.Errorf(format, args...)
	l.emit("error", fmt.Sprintf(format, args...))
}
