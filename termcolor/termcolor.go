// Package termcolor asks the terminal for its theme colors (OSC 10, 11 and 4)
// and derives the rest by mixing them, so a tool follows whatever theme the
// user picked instead of shipping a palette of its own.
//
// The package does not read the terminal itself. A program that owns stdin
// (a viewer with raw input) passes the OSC replies it decodes to Theme.Apply.
// Wait does the retry loop: a new Rex pane may not be attached to the app yet,
// so the first queries can go unanswered.
package termcolor

import (
	"context"
	"fmt"
	"image/color"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
)

// Palette indexes for the ANSI colors a tool most often wants.
const (
	Red    = 1
	Green  = 2
	Yellow = 3
	Blue   = 4
)

// Theme is what the terminal has reported so far.
type Theme struct {
	FG, BG  color.RGBA
	HasFG   bool
	HasBG   bool
	Palette map[int]color.RGBA
}

// Query returns the sequence that asks for the foreground, background and
// the given palette entries. Replies end in BEL, which Rex answers.
func Query(palette ...int) string {
	var b strings.Builder
	b.WriteString("\x1b]10;?\x07\x1b]11;?\x07")
	for _, n := range palette {
		fmt.Fprintf(&b, "\x1b]4;%d;?\x07", n)
	}
	return b.String()
}

// Apply records one OSC reply body, such as "11;rgb:1e1e/1f1f/2222", and
// reports whether it was a color reply. The body is what sits between
// "ESC ]" and the terminator.
func (t *Theme) Apply(body string) bool {
	i := strings.Index(body, ";rgb:")
	if i < 0 {
		return false
	}
	c, ok := ParseRGB(body[i+5:])
	if !ok {
		return false
	}
	head := body[:i]
	switch {
	case head == "10":
		t.FG, t.HasFG = c, true
	case head == "11":
		t.BG, t.HasBG = c, true
	case strings.HasPrefix(head, "4;"):
		n, err := strconv.Atoi(head[2:])
		if err != nil {
			return false
		}
		if t.Palette == nil {
			t.Palette = map[int]color.RGBA{}
		}
		t.Palette[n] = c
	default:
		return false
	}
	return true
}

// Has reports whether the foreground, background and every listed palette
// entry have arrived.
func (t *Theme) Has(palette ...int) bool {
	if !t.HasFG || !t.HasBG {
		return false
	}
	for _, n := range palette {
		if _, ok := t.Palette[n]; !ok {
			return false
		}
	}
	return true
}

// Color returns palette entry n, or fallback if the terminal has not sent it.
func (t *Theme) Color(n int, fallback color.RGBA) color.RGBA {
	if c, ok := t.Palette[n]; ok {
		return c
	}
	return fallback
}

// Dark reports whether the background is dark. Without a background it
// reports false.
func (t *Theme) Dark() bool { return t.HasBG && Luminance(t.BG) < 0.18 }

// Retry timing for Wait: ask every 250ms for up to 3s.
const (
	RetryEvery = 250 * time.Millisecond
	RetryFor   = 3 * time.Second
)

// Wait writes the query to w and applies replies from the channel until the
// theme has the foreground, background and the listed palette entries,
// asking again every RetryEvery. It gives up after RetryFor and returns
// whatever arrived. Bodies that are not color replies are passed to other,
// if it is not nil, so a key pressed during startup is not lost. It returns
// ctx.Err() if ctx ends first, and io.EOF if the channel closes.
func Wait(ctx context.Context, w io.Writer, replies <-chan string, t *Theme, other func(string), palette ...int) error {
	q := Query(palette...)
	if _, err := io.WriteString(w, q); err != nil {
		return err
	}
	retry := time.NewTicker(RetryEvery)
	defer retry.Stop()
	deadline := time.After(RetryFor)
	for !t.Has(palette...) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case body, ok := <-replies:
			if !ok {
				return io.EOF
			}
			if !t.Apply(body) && other != nil {
				other(body)
			}
		case <-retry.C:
			if _, err := io.WriteString(w, q); err != nil {
				return err
			}
		case <-deadline:
			return nil
		}
	}
	return nil
}

// ParseRGB reads an X11 color spec such as "ffff/8080/0000", with 1 to 4
// hex digits per channel.
func ParseRGB(spec string) (color.RGBA, bool) {
	parts := strings.Split(strings.TrimSpace(spec), "/")
	if len(parts) != 3 {
		return color.RGBA{}, false
	}
	var out [3]uint8
	for i, p := range parts {
		if len(p) == 0 || len(p) > 4 {
			return color.RGBA{}, false
		}
		v, err := strconv.ParseUint(p, 16, 16)
		if err != nil {
			return color.RGBA{}, false
		}
		max := float64(uint64(1)<<(4*len(p)) - 1)
		out[i] = uint8(float64(v)/max*255 + 0.5)
	}
	return color.RGBA{out[0], out[1], out[2], 255}, true
}

// Mix returns a moved toward b by t (0 gives a, 1 gives b). Greys for a
// theme come from mixing the foreground into the background.
func Mix(a, b color.RGBA, t float64) color.RGBA {
	l := func(x, y uint8) uint8 { return uint8(math.Round(float64(x) + (float64(y)-float64(x))*t)) }
	return color.RGBA{l(a.R, b.R), l(a.G, b.G), l(a.B, b.B), 255}
}

// Luminance is the relative luminance of c, from 0 for black to 1 for white.
func Luminance(c color.RGBA) float64 {
	lin := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

// Contrast is the WCAG contrast ratio between two colors, from 1 to 21.
func Contrast(a, b color.RGBA) float64 {
	la, lb := Luminance(a)+0.05, Luminance(b)+0.05
	return math.Max(la, lb) / math.Min(la, lb)
}
