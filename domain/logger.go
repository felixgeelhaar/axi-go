package domain

// Logger is the port interface for structured logging.
// Implementations can wrap slog, zap, zerolog, or any logging library.
type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
}

// Field is a key-value pair for structured logging.
type Field struct {
	Key   string
	Value any
}

// F creates a logging field.
func F(key string, value any) Field {
	return Field{Key: key, Value: value}
}

// NopLogger discards all log output. Used as default.
type NopLogger struct{}

func (n *NopLogger) Debug(_ string, _ ...Field) {}
func (n *NopLogger) Info(_ string, _ ...Field)  {}
func (n *NopLogger) Warn(_ string, _ ...Field)  {}
func (n *NopLogger) Error(_ string, _ ...Field) {}
