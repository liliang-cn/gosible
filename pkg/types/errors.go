package types

import (
	"errors"
	"fmt"
)

// Common error types
var (
	ErrHostNotFound     = errors.New("host not found")
	ErrGroupNotFound    = errors.New("group not found")
	ErrModuleNotFound   = errors.New("module not found")
	ErrConnectionFailed = errors.New("connection failed")
	ErrExecutionFailed  = errors.New("execution failed")
	ErrInvalidArguments = errors.New("invalid arguments")
	ErrTimeout          = errors.New("operation timed out")
	ErrPermissionDenied = errors.New("permission denied")
	ErrFileNotFound     = errors.New("file not found")
	ErrTemplateFailed   = errors.New("template rendering failed")
	ErrInventoryFailed  = errors.New("inventory operation failed")
	ErrPlaybookFailed   = errors.New("playbook parsing failed")
)

// ModuleError represents an error from module execution
type ModuleError struct {
	Module  string
	Host    string
	Message string
	Cause   error
}

func (e *ModuleError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("module %s on host %s: %s: %v", e.Module, e.Host, e.Message, e.Cause)
	}
	return fmt.Sprintf("module %s on host %s: %s", e.Module, e.Host, e.Message)
}

func (e *ModuleError) Unwrap() error {
	return e.Cause
}

// ConnectionError represents a connection-related error
type ConnectionError struct {
	Host    string
	Message string
	Cause   error
}

func (e *ConnectionError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("connection to %s: %s: %v", e.Host, e.Message, e.Cause)
	}
	return fmt.Sprintf("connection to %s: %s", e.Host, e.Message)
}

func (e *ConnectionError) Unwrap() error {
	return e.Cause
}

// InventoryError represents an inventory-related error
type InventoryError struct {
	Source  string
	Message string
	Cause   error
}

func (e *InventoryError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("inventory %s: %s: %v", e.Source, e.Message, e.Cause)
	}
	return fmt.Sprintf("inventory %s: %s", e.Source, e.Message)
}

func (e *InventoryError) Unwrap() error {
	return e.Cause
}

// PlaybookError represents a playbook-related error
type PlaybookError struct {
	Playbook string
	Play     string
	Task     string
	Message  string
	Cause    error
}

func (e *PlaybookError) Error() string {
	location := e.Playbook
	if e.Play != "" {
		location = fmt.Sprintf("%s[%s]", location, e.Play)
	}
	if e.Task != "" {
		location = fmt.Sprintf("%s[%s]", location, e.Task)
	}
	
	if e.Cause != nil {
		return fmt.Sprintf("playbook %s: %s: %v", location, e.Message, e.Cause)
	}
	return fmt.Sprintf("playbook %s: %s", location, e.Message)
}

func (e *PlaybookError) Unwrap() error {
	return e.Cause
}

// TemplateError represents a template-related error
type TemplateError struct {
	Template string
	Line     int
	Column   int
	Message  string
	Cause    error
}

func (e *TemplateError) Error() string {
	location := e.Template
	if e.Line > 0 {
		if e.Column > 0 {
			location = fmt.Sprintf("%s:%d:%d", location, e.Line, e.Column)
		} else {
			location = fmt.Sprintf("%s:%d", location, e.Line)
		}
	}
	
	if e.Cause != nil {
		return fmt.Sprintf("template %s: %s: %v", location, e.Message, e.Cause)
	}
	return fmt.Sprintf("template %s: %s", location, e.Message)
}

func (e *TemplateError) Unwrap() error {
	return e.Cause
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Value   interface{}
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed for field %s (value: %v): %s", e.Field, e.Value, e.Message)
}

// NewModuleError creates a new module error
func NewModuleError(module, host, message string, cause error) *ModuleError {
	return &ModuleError{
		Module:  module,
		Host:    host,
		Message: message,
		Cause:   cause,
	}
}

// NewConnectionError creates a new connection error
func NewConnectionError(host, message string, cause error) *ConnectionError {
	return &ConnectionError{
		Host:    host,
		Message: message,
		Cause:   cause,
	}
}

// NewInventoryError creates a new inventory error
func NewInventoryError(source, message string, cause error) *InventoryError {
	return &InventoryError{
		Source:  source,
		Message: message,
		Cause:   cause,
	}
}

// NewPlaybookError creates a new playbook error
func NewPlaybookError(playbook, play, task, message string, cause error) *PlaybookError {
	return &PlaybookError{
		Playbook: playbook,
		Play:     play,
		Task:     task,
		Message:  message,
		Cause:    cause,
	}
}

// NewTemplateError creates a new template error
func NewTemplateError(template string, line, column int, message string, cause error) *TemplateError {
	return &TemplateError{
		Template: template,
		Line:     line,
		Column:   column,
		Message:  message,
		Cause:    cause,
	}
}

// NewValidationError creates a new validation error
func NewValidationError(field string, value interface{}, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Value:   value,
		Message: message,
	}
}