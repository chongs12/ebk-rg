package logger

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
	oteltrace "go.opentelemetry.io/otel/trace"
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
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "timestamp",
			logrus.FieldKeyMsg:   "message",
			logrus.FieldKeyLevel: "level",
		},
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
	if len(args) > 0 {
		if ctx, ok := args[0].(context.Context); ok {
			var msg string
			var kv []interface{}
			if len(args) >= 2 {
				if s, ok := args[1].(string); ok {
					msg = s
					kv = args[2:]
				} else {
					kv = args[1:]
				}
			}
			fields := mergeFields(ctx, kv...)
			Get().WithFields(fields).Info(msg)
			return
		}
	}
	Get().Info(args...)
}

func Infof(format string, args ...interface{}) {
	Get().Infof(format, args...)
}

func Warn(args ...interface{}) {
	if len(args) > 0 {
		if ctx, ok := args[0].(context.Context); ok {
			var msg string
			var kv []interface{}
			if len(args) >= 2 {
				if s, ok := args[1].(string); ok {
					msg = s
					kv = args[2:]
				} else {
					kv = args[1:]
				}
			}
			fields := mergeFields(ctx, kv...)
			Get().WithFields(fields).Warn(msg)
			return
		}
	}
	Get().Warn(args...)
}

func Warnf(format string, args ...interface{}) {
	Get().Warnf(format, args...)
}

func Error(args ...interface{}) {
	if len(args) > 0 {
		if ctx, ok := args[0].(context.Context); ok {
			var msg string
			var kv []interface{}
			if len(args) >= 2 {
				if s, ok := args[1].(string); ok {
					msg = s
					kv = args[2:]
				} else {
					kv = args[1:]
				}
			}
			fields := mergeFields(ctx, kv...)
			Get().WithFields(fields).Error(msg)
			return
		}
	}
	Get().Error(args...)
}

func Errorf(format string, args ...interface{}) {
	Get().Errorf(format, args...)
}

func Fatal(args ...interface{}) {
	if len(args) > 0 {
		if ctx, ok := args[0].(context.Context); ok {
			var msg string
			var kv []interface{}
			if len(args) >= 2 {
				if s, ok := args[1].(string); ok {
					msg = s
					kv = args[2:]
				} else {
					kv = args[1:]
				}
			}
			fields := mergeFields(ctx, kv...)
			Get().WithFields(fields).Fatal(msg)
			return
		}
	}
	Get().Fatal(args...)
}

func Fatalf(format string, args ...interface{}) {
	Get().Fatalf(format, args...)
}

func Panic(args ...interface{}) {
	if len(args) > 0 {
		if ctx, ok := args[0].(context.Context); ok {
			var msg string
			var kv []interface{}
			if len(args) >= 2 {
				if s, ok := args[1].(string); ok {
					msg = s
					kv = args[2:]
				} else {
					kv = args[1:]
				}
			}
			fields := mergeFields(ctx, kv...)
			Get().WithFields(fields).Panic(msg)
			return
		}
	}
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

func WithContext(ctx context.Context) *logrus.Entry {
	fields := fieldsFromContext(ctx)
	if _, ok := fields["event_type"]; !ok {
		fields["event_type"] = "general"
	}
	return Get().WithFields(fields)
}

func WithFieldsCtx(ctx context.Context, fields logrus.Fields) *logrus.Entry {
	f := fieldsFromContext(ctx)
	for k, v := range fields {
		f[k] = v
	}
	if _, ok := f["event_type"]; !ok {
		f["event_type"] = "general"
	}
	return Get().WithFields(f)
}

func fieldsFromContext(ctx context.Context) logrus.Fields {
	f := logrus.Fields{}
	if ctx == nil {
		return f
	}
	if v := ctx.Value("request_id"); v != nil {
		f["request_id"] = v
	}
	if v := ctx.Value("user_id"); v != nil {
		f["user_id"] = v
	}
	sp := oteltrace.SpanFromContext(ctx)
	sc := sp.SpanContext()
	if sc.TraceID().IsValid() {
		f["trace_id"] = sc.TraceID().String()
	}
	if sc.SpanID().IsValid() {
		f["span_id"] = sc.SpanID().String()
	}
	return f
}

func kvToFields(kv ...interface{}) logrus.Fields {
	f := logrus.Fields{}
	for i := 0; i+1 < len(kv); i += 2 {
		k := fmt.Sprint(kv[i])
		f[k] = kv[i+1]
	}
	return f
}

func mergeFields(ctx context.Context, kv ...interface{}) logrus.Fields {
	f := fieldsFromContext(ctx)
	for k, v := range kvToFields(kv...) {
		f[k] = v
	}
	if _, ok := f["event_type"]; !ok {
		f["event_type"] = "general"
	}
	return f
}
