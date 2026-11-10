package termcolor

import (
	"bytes"
	"context"
	"image/color"
	"strings"
	"testing"
	"time"
)

func TestParseRGB(t *testing.T) {
	tests := []struct {
		in   string
		want color.RGBA
		ok   bool
	}{
		{"ffff/8080/0000", color.RGBA{255, 128, 0, 255}, true},
		{"ff/80/00", color.RGBA{255, 128, 0, 255}, true},
		{"f/8/0", color.RGBA{255, 136, 0, 255}, true},
		{"1e1e/1f1f/2222", color.RGBA{30, 31, 34, 255}, true},
		{"ff/80", color.RGBA{}, false},
		{"ffff/zz/00", color.RGBA{}, false},
		{"fffff/00/00", color.RGBA{}, false},
	}
	for _, tt := range tests {
		got, ok := ParseRGB(tt.in)
		if ok != tt.ok || got != tt.want {
			t.Errorf("ParseRGB(%q) = %v, %v; want %v, %v", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestApply(t *testing.T) {
	var th Theme
	for _, body := range []string{"10;rgb:ffff/ffff/ffff", "11;rgb:0000/0000/0000", "4;4;rgb:0000/0000/ffff"} {
		if !th.Apply(body) {
			t.Errorf("Apply(%q) = false", body)
		}
	}
	if th.Apply("52;c;aGVsbG8=") || th.Apply("4;x;rgb:0/0/0") {
		t.Error("Apply accepted a non-color reply")
	}
	if !th.Has(Blue) || th.Has(Red) {
		t.Error("Has is wrong")
	}
	if !th.Dark() {
		t.Error("black background should be dark")
	}
	if got := th.Color(Red, color.RGBA{1, 2, 3, 255}); got != (color.RGBA{1, 2, 3, 255}) {
		t.Errorf("fallback not used: %v", got)
	}
}

func TestQuery(t *testing.T) {
	want := "\x1b]10;?\x07\x1b]11;?\x07\x1b]4;1;?\x07\x1b]4;4;?\x07"
	if got := Query(Red, Blue); got != want {
		t.Errorf("got %q", got)
	}
}

// Wait asks again until the terminal answers, the way a new Rex pane behaves
// before the app attaches.
func TestWaitRetries(t *testing.T) {
	var out bytes.Buffer
	replies := make(chan string)
	go func() {
		time.Sleep(RetryEvery + 50*time.Millisecond) // the first query goes unanswered
		replies <- "10;rgb:eeee/eeee/eeee"
		replies <- "200~" // not a color: handed to other
		replies <- "11;rgb:1111/1111/1111"
		replies <- "4;4;rgb:3333/6666/ffff"
	}()
	var th Theme
	var others []string
	err := Wait(context.Background(), &out, replies, &th, func(s string) { others = append(others, s) }, Blue)
	if err != nil {
		t.Fatal(err)
	}
	if !th.Has(Blue) {
		t.Fatal("theme incomplete")
	}
	if n := strings.Count(out.String(), "\x1b]11;?"); n < 2 {
		t.Errorf("asked %d times, want a retry", n)
	}
	if len(others) != 1 || others[0] != "200~" {
		t.Errorf("others = %q", others)
	}
}

func TestMixAndContrast(t *testing.T) {
	black, white := color.RGBA{0, 0, 0, 255}, color.RGBA{255, 255, 255, 255}
	if got := Mix(black, white, 0.5); got != (color.RGBA{128, 128, 128, 255}) {
		t.Errorf("Mix = %v", got)
	}
	if c := Contrast(black, white); c < 20.9 || c > 21.1 {
		t.Errorf("Contrast = %v", c)
	}
}
