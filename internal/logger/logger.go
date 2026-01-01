package logger

import (
	"strings"

	"github.com/sirupsen/logrus"
)

type Logger struct {
	log *logrus.Logger
}

func New(level string) *Logger {
	log := logrus.New()
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	switch strings.ToLower(level) {
	case "debug":
		log.SetLevel(logrus.DebugLevel)
	case "warn":
		log.SetLevel(logrus.WarnLevel)
	case "error":
		log.SetLevel(logrus.ErrorLevel)
	default:
		log.SetLevel(logrus.InfoLevel)
	}

	return &Logger{log: log}
}

func (l *Logger) Info(msg string) {
	l.log.Info(msg)
}
