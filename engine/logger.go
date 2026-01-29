package engine

import (
	"log"
	"os"
)

type NopLogger struct{}

func (NopLogger) Printf(format string, args ...any) {}

func StdLogger() Logger {
	return log.New(os.Stdout, "[corm] ", log.LstdFlags)
}
