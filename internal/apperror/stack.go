package apperror

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
)

// Error carries an operation name, optional typed category and a stack trace.
// It supports errors.Is through Is and Unwrap.
type Error struct {
	Kind  error
	Op    string
	Msg   string
	Err   error
	stack []uintptr
}

// New creates an Error with a stack trace.
func New(kind error, op, msg string) error {
	return &Error{
		Kind:  kind,
		Op:    strings.TrimSpace(op),
		Msg:   strings.TrimSpace(msg),
		stack: captureStack(3),
	}
}

// Wrap adds operation context and a stack trace around err.
func Wrap(op string, err error) error {
	if err == nil {
		return nil
	}
	return &Error{
		Op:    strings.TrimSpace(op),
		Err:   err,
		stack: captureStack(3),
	}
}

// WrapKind adds a typed category, operation context, and stack trace around err.
func WrapKind(kind error, op string, err error) error {
	if err == nil {
		return nil
	}
	return &Error{
		Kind:  kind,
		Op:    strings.TrimSpace(op),
		Err:   err,
		stack: captureStack(3),
	}
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}

	parts := make([]string, 0, 4)
	if e.Op != "" {
		parts = append(parts, e.Op)
	}
	if e.Msg != "" {
		parts = append(parts, e.Msg)
	}
	if e.Err != nil {
		parts = append(parts, e.Err.Error())
	} else if e.Kind != nil {
		parts = append(parts, e.Kind.Error())
	}
	if len(parts) == 0 {
		return "application error"
	}
	return strings.Join(parts, ": ")
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	if e.Err != nil {
		return e.Err
	}
	return e.Kind
}

func (e *Error) Is(target error) bool {
	if e == nil {
		return false
	}
	if e.Kind != nil && errors.Is(e.Kind, target) {
		return true
	}
	if e.Err != nil && errors.Is(e.Err, target) {
		return true
	}
	return false
}

// Stack returns a formatted copy of the captured stack frames.
func (e *Error) Stack() []string {
	if e == nil || len(e.stack) == 0 {
		return nil
	}
	frames := runtime.CallersFrames(e.stack)
	out := make([]string, 0, len(e.stack))
	for {
		frame, more := frames.Next()
		out = append(out, fmt.Sprintf("%s (%s:%d)", frame.Function, frame.File, frame.Line))
		if !more {
			break
		}
	}
	return out
}

func captureStack(skip int) []uintptr {
	pcs := make([]uintptr, 32)
	n := runtime.Callers(skip, pcs)
	if n == 0 {
		return nil
	}
	stack := make([]uintptr, n)
	copy(stack, pcs[:n])
	return stack
}
