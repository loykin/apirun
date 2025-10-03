package main

import (
	"os"

	"github.com/loykin/apirun/internal/common"
)

// ExitHandler provides a testable way to handle program termination
type ExitHandler interface {
	Exit(code int)
	LogFatalError(err error, msg string, keyvals ...any)
}

// DefaultExitHandler implements ExitHandler for production use
type DefaultExitHandler struct {
	logger *common.Logger
}

// NewDefaultExitHandler creates a new default exit handler
func NewDefaultExitHandler() *DefaultExitHandler {
	return &DefaultExitHandler{
		logger: common.GetLogger().WithComponent("main"),
	}
}

// Exit terminates the program with the given exit code
func (h *DefaultExitHandler) Exit(code int) {
	os.Exit(code)
}

// LogFatalError logs a fatal error and exits the program
func (h *DefaultExitHandler) LogFatalError(err error, msg string, keyvals ...any) {
	// Combine error with additional key-value pairs
	allKeyvals := append([]any{"error", err}, keyvals...)
	h.logger.Error(msg, allKeyvals...)
	h.Exit(1)
}

// Global exit handler (can be replaced for testing)
var exitHandler ExitHandler = NewDefaultExitHandler()
