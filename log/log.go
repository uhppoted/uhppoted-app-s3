package log

import (
	syslog "log"

	"github.com/uhppoted/uhppoted-lib/log"
)

type LogLevel int

func (l LogLevel) String() string {
	return []string{"NONE", "DEBUG", "INFO", "WARN", "ERROR"}[l]
}

func SetDebug(enabled bool) {
	log.SetDebug(enabled)
}

func SetLevel(level string) {
	log.SetLevel(level)
}

func SetLogger(logger *syslog.Logger) {
	log.SetLogger(logger)
}

func SetFatalHook(f func()) {
	log.AddFatalHook(f)
}

func Debugf(format string, args ...any) {
	log.Debugf(format, args...)
}

func Infof(format string, args ...any) {
	log.Infof(format, args...)
}

func Warnf(format string, args ...any) {
	log.Warnf(format, args...)
}

func Errorf(format string, args ...any) {
	log.Errorf(format, args...)
}

func Fatalf(format string, args ...any) {
	log.Fatalf(format, args...)
}
