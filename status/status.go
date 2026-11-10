// Package status writes OSC 7501 program status reports, the escape sequence
// a program uses to tell its terminal that it is idle, working, blocked on
// the user, done or failed.
//
// Spec: https://www.superlogical.com/rex/docs/build/program-status
package status

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
)

// State is what a program says it is doing.
type State string

const (
	Idle    State = "idle"
	Working State = "working"
	Done    State = "done" // a result the user has not seen yet
	Blocked State = "blocked"
	Error   State = "error"
	Clear   State = "clear" // not a state: removes the record and its children
)

// Kind says what a blocked program is waiting for.
type Kind string

const (
	Permission Kind = "permission"
	Question   Kind = "question"
	Auth       Kind = "auth"
)

// Report is one status report. Each report replaces its record completely, so
// set every field the record should keep.
type Report struct {
	State    State
	ID       string // record path such as "build/test", empty for the root record
	App      string // the program's name
	Kind     Kind   // only for Blocked
	Msg      string // one line of plain text
	Title    string // a short label for the record
	Progress *int   // 0 to 100, nil to leave it out. Working and Blocked only.
}

// Percent returns a progress value for Report.Progress.
func Percent(n int) *int { return &n }

// Encode returns the escape sequence for r, terminated with ST.
func (r Report) Encode() (string, error) {
	switch r.State {
	case Idle, Working, Done, Blocked, Error, Clear:
	default:
		return "", fmt.Errorf("status: unknown state %q", r.State)
	}
	pairs := []string{"state=" + string(r.State)}
	add := func(key, val string) error {
		if val == "" {
			return nil
		}
		if strings.ContainsAny(val, ":;") || hasControl(val) {
			return fmt.Errorf("status: %s %q may not contain ':', ';' or control characters", key, val)
		}
		pairs = append(pairs, key+"="+val)
		return nil
	}
	text := func(key, val string) error {
		if val == "" {
			return nil
		}
		if hasControl(val) {
			return fmt.Errorf("status: %s must be one line without control characters", key)
		}
		pairs = append(pairs, key+"="+base64.StdEncoding.EncodeToString([]byte(val)))
		return nil
	}
	if err := errors.Join(add("id", r.ID), add("app", r.App), add("kind", string(r.Kind)),
		text("msg", r.Msg), text("title", r.Title)); err != nil {
		return "", err
	}
	if r.Progress != nil && (r.State == Working || r.State == Blocked) {
		pairs = append(pairs, "progress="+strconv.Itoa(max(0, min(*r.Progress, 100))))
	}
	return "\x1b]7501;" + strings.Join(pairs, ":") + "\x1b\\", nil
}

// Write encodes r and writes it to w.
func (r Report) Write(w io.Writer) error {
	s, err := r.Encode()
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, s)
	return err
}

// Query is the feature detection sequence. A terminal that supports the
// protocol answers with OSC 7501 ; ? and optional pairs.
const Query = "\x1b]7501;?\x1b\\"

func hasControl(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}
