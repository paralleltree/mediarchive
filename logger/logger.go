package logger

import (
	"fmt"
	"os"
)

type Logger interface {
	Print(format string, args ...interface{})
	Info(format string, args ...interface{})
}

type logger struct{}

func NewLogger() *logger {
	return &logger{}
}

// Prints text to stdout
func (l *logger) Print(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

// Prints text to stderr
func (l *logger) Info(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
}
