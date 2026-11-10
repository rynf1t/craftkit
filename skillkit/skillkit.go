// Package skillkit installs an agent skill that ships inside a tool's binary,
// so the skill can never drift from the tool that reads it.
//
// The installed file carries a marker line naming the tool. Install only
// touches files with that marker and reports what it found:
//
//	created    no file was there, and Install wrote one
//	unchanged  the installed file already matches
//	outdated   an older copy from this tool was there, and Install replaced it
//	foreign    a file the tool did not write is there, and Install left it alone
package skillkit

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// State is what Check found at the skill's path.
type State string

const (
	Missing   State = "missing"
	Created   State = "created"
	Unchanged State = "unchanged"
	Outdated  State = "outdated"
	Foreign   State = "foreign"
)

// Skill is one embedded SKILL.md.
type Skill struct {
	Name    string // folder name, such as "radar"
	Tool    string // the command that installs it, such as "radar"
	Content []byte // the SKILL.md as embedded, frontmatter first
}

// Result says what Install or Remove found and whether it wrote.
type Result struct {
	Path  string `json:"path"`
	State State  `json:"state"` // what was there before
	Wrote bool   `json:"wrote"` // whether the file changed (false on a dry run)
}

// DefaultDir is where Claude Code looks for personal skills.
func DefaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".claude", "skills")
	}
	return filepath.Join(home, ".claude", "skills")
}

// Path is where the skill lives under dir.
func (s Skill) Path(dir string) string { return filepath.Join(dir, s.Name, "SKILL.md") }

func (s Skill) marker() []byte {
	return fmt.Appendf(nil, "<!-- Installed by \"%s skill install\". Remove it with \"%s skill remove\". -->\n", s.Tool, s.Tool)
}

func (s Skill) markerPrefix() []byte {
	return fmt.Appendf(nil, "<!-- Installed by \"%s skill install\".", s.Tool)
}

// Rendered is the file Install writes: the frontmatter, then the marker,
// then the body.
func (s Skill) Rendered() []byte {
	head, body := splitFrontmatter(s.Content)
	var b bytes.Buffer
	b.Write(head)
	if len(head) > 0 {
		b.WriteByte('\n')
	}
	b.Write(s.marker())
	if len(body) > 0 && body[0] != '\n' {
		b.WriteByte('\n')
	}
	b.Write(body)
	return b.Bytes()
}

// Check reports what is at the skill's path under dir.
func (s Skill) Check(dir string) (State, error) {
	have, err := os.ReadFile(s.Path(dir))
	if errors.Is(err, fs.ErrNotExist) {
		return Missing, nil
	}
	if err != nil {
		return "", err
	}
	switch {
	case bytes.Equal(have, s.Rendered()):
		return Unchanged, nil
	case bytes.Contains(have, s.markerPrefix()):
		return Outdated, nil
	}
	return Foreign, nil
}

// Install writes the skill under dir unless it is unchanged or foreign.
// force replaces a foreign file. dryRun reports without writing. The State
// in the result is Created for a missing file.
func (s Skill) Install(dir string, dryRun, force bool) (Result, error) {
	path := s.Path(dir)
	st, err := s.Check(dir)
	if err != nil {
		return Result{Path: path}, err
	}
	res := Result{Path: path, State: st}
	if st == Missing {
		res.State = Created
	}
	if st == Unchanged || (st == Foreign && !force) || dryRun {
		return res, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return res, err
	}
	if err := writeAtomic(path, s.Rendered()); err != nil {
		return res, err
	}
	res.Wrote = true
	return res, nil
}

// Remove deletes the skill's file and folder if this tool installed them.
// It leaves a foreign file alone and does nothing for a missing one.
func (s Skill) Remove(dir string, dryRun bool) (Result, error) {
	path := s.Path(dir)
	st, err := s.Check(dir)
	res := Result{Path: path, State: st}
	if err != nil || st == Missing || st == Foreign || dryRun {
		return res, err
	}
	if err := os.Remove(path); err != nil {
		return res, err
	}
	os.Remove(filepath.Dir(path)) // only if empty
	res.Wrote = true
	return res, nil
}

// Describe is a one-line report of res for people, such as
// "updated ~/.claude/skills/radar/SKILL.md (was outdated)".
func (s Skill) Describe(res Result, removing bool) string {
	p := res.Path
	switch {
	case res.State == Foreign:
		verb := "replace"
		if removing {
			verb = "remove"
		}
		if res.Wrote {
			return fmt.Sprintf("replaced %s (was foreign)", p)
		}
		return fmt.Sprintf("foreign %s: not installed by %s, left alone (pass --force to %s it)", p, s.Tool, verb)
	case removing && res.State == Missing:
		return "missing " + p
	case removing && res.Wrote:
		return "removed " + p
	case removing:
		return "would remove " + p
	case res.State == Unchanged:
		return "unchanged " + p
	case res.State == Outdated && res.Wrote:
		return fmt.Sprintf("updated %s (was outdated)", p)
	case res.State == Outdated:
		return "outdated " + p
	case res.State == Created && res.Wrote:
		return "created " + p
	}
	return "would create " + p
}

// splitFrontmatter returns the leading "---" block (with both fences) and
// the rest. Without frontmatter, head is empty.
func splitFrontmatter(b []byte) (head, body []byte) {
	if !bytes.HasPrefix(b, []byte("---\n")) {
		return nil, b
	}
	end := bytes.Index(b[4:], []byte("\n---\n"))
	if end < 0 {
		return nil, b
	}
	cut := 4 + end + len("\n---")
	return b[:cut], b[cut+1:]
}

func writeAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".skill-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
