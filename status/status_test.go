package status

import (
	"bytes"
	"testing"
)

func TestEncode(t *testing.T) {
	tests := []struct {
		name string
		r    Report
		want string
	}{
		{"root working", Report{State: Working, App: "radar"},
			"\x1b]7501;state=working:app=radar\x1b\\"},
		{"blocked with message", Report{State: Blocked, App: "claude", Kind: Permission, Msg: "Run make?"},
			"\x1b]7501;state=blocked:app=claude:kind=permission:msg=UnVuIG1ha2U/\x1b\\"},
		{"child with progress", Report{State: Working, ID: "build/test", Progress: Percent(40)},
			"\x1b]7501;state=working:id=build/test:progress=40\x1b\\"},
		{"progress capped", Report{State: Working, Progress: Percent(250)},
			"\x1b]7501;state=working:progress=100\x1b\\"},
		{"progress dropped on done", Report{State: Done, Progress: Percent(40)},
			"\x1b]7501;state=done\x1b\\"},
		{"title", Report{State: Done, Title: "Tests"},
			"\x1b]7501;state=done:title=VGVzdHM=\x1b\\"},
		{"clear", Report{State: Clear}, "\x1b]7501;state=clear\x1b\\"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.r.Encode()
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEncodeRejects(t *testing.T) {
	bad := []Report{
		{State: "sleeping"},
		{State: Working, App: "a:b"},
		{State: Working, ID: "x;y"},
		{State: Blocked, Msg: "two\nlines"},
		{State: Done, Title: "bell\a"},
	}
	for _, r := range bad {
		if _, err := r.Encode(); err == nil {
			t.Errorf("Encode(%+v) succeeded, want an error", r)
		}
	}
}

func TestWrite(t *testing.T) {
	var b bytes.Buffer
	if err := (Report{State: Idle}).Write(&b); err != nil {
		t.Fatal(err)
	}
	if b.String() != "\x1b]7501;state=idle\x1b\\" {
		t.Errorf("wrote %q", b.String())
	}
}
