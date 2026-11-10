// Package cliout holds the output conventions every craftkit CLI shares: one
// exit code table, errors that carry their code, and JSON that is indented
// for people and compact for pipes.
package cliout

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

// Exit codes. Scripts branch on these, so they never change meaning.
const (
	OK          = 0   // success
	Failure     = 1   // the command ran and failed
	Usage       = 2   // invalid command line
	NotFound    = 3   // target not found or ambiguous
	Unreachable = 4   // server or dependency unreachable
	Interrupted = 130 // interrupted by the user
)

// Error is an error with an exit code.
type Error struct {
	Code int
	Err  error
}

func (e *Error) Error() string { return e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }

// Errorf returns an *Error with code and a formatted message.
func Errorf(code int, format string, args ...any) error {
	return &Error{Code: code, Err: fmt.Errorf(format, args...)}
}

// Wrap gives err an exit code. It returns nil for a nil err.
func Wrap(code int, err error) error {
	if err == nil {
		return nil
	}
	return &Error{Code: code, Err: err}
}

// Code is the exit code for err: OK for nil, the code an *Error carries,
// Interrupted for a cancelled context, and Failure otherwise.
func Code(err error) int {
	if err == nil {
		return OK
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	if errors.Is(err, context.Canceled) {
		return Interrupted
	}
	return Failure
}

// Fail prints err to w as "prog: message" and returns its exit code. It
// prints nothing for nil.
func Fail(w io.Writer, prog string, err error) int {
	if err != nil {
		fmt.Fprintf(w, "%s: %v\n", prog, err)
	}
	return Code(err)
}

// IsTerminal reports whether w is a terminal.
func IsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// JSON writes v to w with two-space indentation when w is a terminal and on
// one line otherwise, followed by a newline.
func JSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if IsTerminal(w) {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(v)
}
