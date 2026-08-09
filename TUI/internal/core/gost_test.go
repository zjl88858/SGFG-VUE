package core

import (
	"os"
	"testing"

	"github.com/go-gost/core/logger"
)

func TestGostLoggerIsSilentByDefault(t *testing.T) {
	if os.Getenv("SGFG_GOST_DEBUG") == "1" {
		t.Skip("debug logging explicitly enabled")
	}
	for _, level := range []logger.LogLevel{
		logger.TraceLevel,
		logger.DebugLevel,
		logger.InfoLevel,
		logger.WarnLevel,
		logger.ErrorLevel,
	} {
		if gostLogger.IsLevelEnabled(level) {
			t.Fatalf("GoST level %q should be disabled", level)
		}
	}
}

func TestDiscardLoggerWithFieldsStaysSilent(t *testing.T) {
	child := (discardLogger{}).WithFields(map[string]any{"client": "127.0.0.1"})
	if child.IsLevelEnabled(logger.InfoLevel) {
		t.Fatal("child logger unexpectedly enabled info logs")
	}
	child.Info("must not be written")
}
