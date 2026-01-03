package logger

import (
	"io"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Config struct {
	Level      string
	Format     string
	Output     string
	MaxSize    int
	MaxBackups int
	MaxAge     int
	Compress   bool
}

type Logger struct {
	log *logrus.Logger
}

func New(cfg Config) *Logger {
	log := logrus.New()

	switch strings.ToLower(cfg.Format) {
	case "json":
		log.SetFormatter(&logrus.JSONFormatter{})
	default:
		log.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02T15:04:05+07:00",
			ForceColors:     true,
		})
	}

	switch strings.ToLower(cfg.Level) {
	case "debug":
		log.SetLevel(logrus.DebugLevel)
	case "warn":
		log.SetLevel(logrus.WarnLevel)
	case "error":
		log.SetLevel(logrus.ErrorLevel)
	case "fatal":
		log.SetLevel(logrus.FatalLevel)
	case "panic":
		log.SetLevel(logrus.PanicLevel)
	default:
		log.SetLevel(logrus.InfoLevel)
	}

	var writer io.Writer
	if cfg.Output != "" && cfg.Output != "stdout" {
		writer = &lumberjack.Logger{
			Filename:   cfg.Output,
			MaxSize:    cfg.MaxSize,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAge,
			Compress:   cfg.Compress,
			LocalTime:  true,
		}
	} else {
		writer = os.Stdout
	}

	log.SetOutput(writer)

	return &Logger{log: log}
}

func (l *Logger) Debug(msg string) {
	l.log.Debug(msg)
}

func (l *Logger) Info(msg string) {
	l.log.Info(msg)
}

func (l *Logger) Warn(msg string) {
	l.log.Warn(msg)
}

func (l *Logger) Error(msg string) {
	l.log.Error(msg)
}

func (l *Logger) Fatal(msg string) {
	l.log.Fatal(msg)
}

func (l *Logger) Panic(msg string) {
	l.log.Panic(msg)
}

func (l *Logger) WithFields(fields logrus.Fields) *logrus.Entry {
	return l.log.WithFields(fields)
}

func (l *Logger) WithError(err error) *logrus.Entry {
	return l.log.WithError(err)
}

func (l *Logger) WithRequestID(requestID string) *logrus.Entry {
	return l.log.WithField("request_id", requestID)
}

func (l *Logger) WithComponent(component string) *logrus.Entry {
	return l.log.WithField("component", component)
}

func (l *Logger) WithSymbol(symbol string) *logrus.Entry {
	return l.log.WithField("symbol", symbol)
}

func (l *Logger) WithOrderID(orderID string) *logrus.Entry {
	return l.log.WithField("order_id", orderID)
}
