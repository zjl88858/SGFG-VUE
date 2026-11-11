package core

import (
	"strings"
	"testing"

	"github.com/go-gost/core/logger"
)

type quietGostLogger struct{ logger.Logger }

func (q *quietGostLogger) WithFields(map[string]any) logger.Logger { return q }
func (*quietGostLogger) Error(...any)                              {}
func (*quietGostLogger) Errorf(string, ...any)                     {}
func (*quietGostLogger) Warn(...any)                               {}
func (*quietGostLogger) Warnf(string, ...any)                      {}

func TestGostErrorForwarding(t *testing.T) {
	var got []string
	SetGostLogSink(func(level, msg string) { got = append(got, level+":"+msg) })
	defer SetGostLogSink(nil)
	l := (&gostEventLogger{Logger: &quietGostLogger{}}).WithFields(map[string]any{"kind": "dialer", "password": "secret"})
	l.Errorf("route: %s", "timeout: no recent network activity")
	l.Warn("cannot dial ss://user:secret@host:8388")
	l.Error("use of closed network connection")
	if len(got) != 2 || !strings.Contains(got[0], "GOST [dialer]: route: timeout: no recent network activity") {
		t.Fatal(got)
	}
	if strings.Contains(got[1], "secret") || !strings.Contains(got[1], "***@host") {
		t.Fatal("credentials were not redacted")
	}
}
