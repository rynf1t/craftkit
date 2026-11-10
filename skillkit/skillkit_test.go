package skillkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const md = "---\nname: radar\ndescription: test\n---\n\n# Radar\n\nBody.\n"

func skill(content string) Skill {
	return Skill{Name: "radar", Tool: "radar", Content: []byte(content)}
}

func TestRenderedKeepsFrontmatterFirst(t *testing.T) {
	got := string(skill(md).Rendered())
	want := "---\nname: radar\ndescription: test\n---\n" +
		"<!-- Installed by \"radar skill install\". Remove it with \"radar skill remove\". -->\n" +
		"\n# Radar\n\nBody.\n"
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestInstallLifecycle(t *testing.T) {
	dir := t.TempDir()
	s := skill(md)

	res, err := s.Install(dir, true, false)
	if err != nil || res.State != Created || res.Wrote {
		t.Fatalf("dry run: %+v %v", res, err)
	}
	if _, err := os.Stat(s.Path(dir)); !os.IsNotExist(err) {
		t.Fatal("dry run wrote a file")
	}

	steps := []struct {
		skill Skill
		state State
		wrote bool
	}{
		{s, Created, true},
		{s, Unchanged, false},
		{skill(strings.Replace(md, "Body.", "New body.", 1)), Outdated, true},
	}
	for _, st := range steps {
		res, err := st.skill.Install(dir, false, false)
		if err != nil || res.State != st.state || res.Wrote != st.wrote {
			t.Fatalf("install: got %+v %v, want %s wrote=%v", res, err, st.state, st.wrote)
		}
	}
	res, err = s.Remove(dir, false)
	if err != nil || !res.Wrote {
		t.Fatalf("remove: %+v %v", res, err)
	}
	if _, err := os.Stat(filepath.Dir(s.Path(dir))); !os.IsNotExist(err) {
		t.Error("remove left the folder")
	}
}

func TestForeignIsLeftAlone(t *testing.T) {
	dir := t.TempDir()
	s := skill(md)
	path := s.Path(dir)
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte("my own notes\n"), 0o644)

	res, err := s.Install(dir, false, false)
	if err != nil || res.State != Foreign || res.Wrote {
		t.Fatalf("install over foreign: %+v %v", res, err)
	}
	if res, _ := s.Remove(dir, false); res.Wrote {
		t.Fatal("remove deleted a foreign file")
	}
	if b, _ := os.ReadFile(path); string(b) != "my own notes\n" {
		t.Fatal("foreign file changed")
	}
	if !strings.Contains(s.Describe(res, true), "left alone") {
		t.Errorf("describe: %s", s.Describe(res, true))
	}
	res, _ = s.Install(dir, false, true)
	if !res.Wrote || res.State != Foreign {
		t.Fatalf("force: %+v", res)
	}
}

func TestDescribe(t *testing.T) {
	s := skill(md)
	cases := map[string]Result{
		"created p":                {Path: "p", State: Created, Wrote: true},
		"would create p":           {Path: "p", State: Created},
		"unchanged p":              {Path: "p", State: Unchanged},
		"updated p (was outdated)": {Path: "p", State: Outdated, Wrote: true},
		"replaced p (was foreign)": {Path: "p", State: Foreign, Wrote: true},
	}
	for want, res := range cases {
		if got := s.Describe(res, false); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}
