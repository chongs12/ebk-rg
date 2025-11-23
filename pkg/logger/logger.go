package logger

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
)

type Logger struct {
	*logrus.Logger
}

var logger *Logger

func Init() *Logger {
	if logger != nil {
		return logger
	}

	log := logrus.New()
	log.SetOutput(os.Stdout)
	log.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02 15:04:05",
		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
			filename := strings.Split(f.File, "/")
			return fmt.Sprintf("%s:%d", filename[len(filename)-1], f.Line), ""
		},
	})

	log.SetReportCaller(true)
	log.SetLevel(logrus.InfoLevel)

	logger = &Logger{log}
	return logger
}

func Get() *Logger {
	if logger == nil {
		return Init()
	}
	return logger
}

func SetLevel(level string) {
	logLevel, err := logrus.ParseLevel(level)
	if err != nil {
		logLevel = logrus.InfoLevel
	}
	Get().SetLevel(logLevel)
}

func (l *Logger) WithField(key string, value interface{}) *logrus.Entry {
	return l.Logger.WithField(key, value)
}

func (l *Logger) WithFields(fields logrus.Fields) *logrus.Entry {
	return l.Logger.WithFields(fields)
}

func (l *Logger) WithError(err error) *logrus.Entry {
	return l.Logger.WithError(err)
}

func Debug(args ...interface{}) {
	Get().Debug(args...)
}

func Debugf(format string, args ...interface{}) {
	Get().Debugf(format, args...)
}

func Info(args ...interface{}) {
	Get().Info(args...)
}

func Infof(format string, args ...interface{}) {
	Get().Infof(format, args...)
}

func Warn(args ...interface{}) {
	Get().Warn(args...)
}

func Warnf(format string, args ...interface{}) {
	Get().Warnf(format, args...)
}

func Error(args ...interface{}) {
	Get().Error(args...)
}

func Errorf(format string, args ...interface{}) {
	Get().Errorf(format, args...)
}

func Fatal(args ...interface{}) {
	Get().Fatal(args...)
}

func Fatalf(format string, args ...interface{}) {
	Get().Fatalf(format, args...)
}

func Panic(args ...interface{}) {
	Get().Panic(args...)
}

func Panicf(format string, args ...interface{}) {
	Get().Panicf(format, args...)
}

func WithField(key string, value interface{}) *logrus.Entry {
	return Get().WithField(key, value)
}

func WithFields(fields logrus.Fields) *logrus.Entry {
	return Get().WithFields(fields)
}

func WithError(err error) *logrus.Entry {
	return Get().WithError(err)
}