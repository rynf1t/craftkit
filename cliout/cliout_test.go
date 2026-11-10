package cliout

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestCode(t *testing.T) {
	tests := []struct {
		err  error
		want int
	}{
		{nil, OK},
		{errors.New("boom"), Failure},
		{Errorf(NotFound, "no agent %q", "x"), NotFound},
		{fmt.Errorf("listing: %w", Errorf(Unreachable, "rex is not running")), Unreachable},
		{context.Canceled, Interrupted},
		{Wrap(Usage, errors.New("bad flag")), Usage},
	}
	for _, tt := range tests {
		if got := Code(tt.err); got != tt.want {
			t.Errorf("Code(%v) = %d, want %d", tt.err, got, tt.want)
		}
	}
	if Wrap(Usage, nil) != nil {
		t.Error("Wrap(nil) should be nil")
	}
}

func TestFail(t *testing.T) {
	var b bytes.Buffer
	if code := Fail(&b, "radar", Errorf(NotFound, "no agent matches %q", "zz")); code != NotFound {
		t.Errorf("code %d", code)
	}
	if got, want := b.String(), "radar: no agent matches \"zz\"\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	b.Reset()
	if Fail(&b, "radar", nil) != OK || b.Len() != 0 {
		t.Error("nil error should print nothing and return OK")
	}
}

func TestJSONCompactWhenPiped(t *testing.T) {
	var b bytes.Buffer
	if err := JSON(&b, map[string]any{"a": 1, "b": "<x>"}); err != nil {
		t.Fatal(err)
	}
	if got, want := b.String(), "{\"a\":1,\"b\":\"<x>\"}\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
