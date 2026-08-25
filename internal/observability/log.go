package observability

import (
	"errors"
	"io"
	"log/slog"
	"strings"
)

type Class string

const (
	ClassAuth          Class = "auth"
	ClassTLS           Class = "tls"
	ClassTimeout       Class = "timeout"
	ClassTransport     Class = "transport"
	ClassHTTP          Class = "http"
	ClassDecode        Class = "decode"
	ClassValidation    Class = "validation"
	ClassKafkaAuth     Class = "kafka_auth"
	ClassKafkaTimeout  Class = "kafka_timeout"
	ClassKafkaRejected Class = "kafka_rejected"
	ClassBufferFull    Class = "buffer_full"
	ClassInternal      Class = "internal"
)

type Classified interface {
	FailureClass() Class
}

func Classify(err error) Class {
	if err == nil {
		return ""
	}
	var classified Classified
	if errors.As(err, &classified) {
		return classified.FailureClass()
	}
	return ClassInternal
}

func NewLogger(output io.Writer, level, format string) *slog.Logger {
	var configuredLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		configuredLevel = slog.LevelDebug
	case "warn":
		configuredLevel = slog.LevelWarn
	case "error":
		configuredLevel = slog.LevelError
	default:
		configuredLevel = slog.LevelInfo
	}
	options := &slog.HandlerOptions{Level: configuredLevel}
	if format == "text" {
		return slog.New(slog.NewTextHandler(output, options))
	}
	return slog.New(slog.NewJSONHandler(output, options))
}

func LogFailure(logger *slog.Logger, message, component string, err error) {
	logger.Error(message, FailureFields(component, err)...)
}
