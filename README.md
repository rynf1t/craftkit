# craftkit

Small Go packages for the things every tool we build needs: following the
terminal's colours, one exit code table, shipping an agent skill inside the
binary, and reporting status to the terminal.

**Project Status:** early. Four packages, each pulled out of a real tool
([radar](https://github.com/rynf1t/agent-radar)) when it needed one. The API
will change while there's only one user.

## Why?

Every CLI and pane I build ends up re-solving the same small problems, and
each copy drifts a little. Rex and rexpad showed the bar: ask the terminal for
its colours instead of shipping a palette, give scripts exit codes they can
branch on, keep the agent skill inside the binary so it can't go stale, and
let programs report their own state. craftkit is that bar as code, so the
next tool starts there.

## Install

```sh
go get github.com/rynf1t/craftkit
```

## Packages

| Package | Does |
|---|---|
| `termcolor` | Asks the terminal for its foreground, background and palette colours (OSC 10, 11, 4), retrying while a new pane attaches. Mixes greys from them. |
| `cliout` | The exit code table, errors that carry a code, and JSON that's indented on a terminal and compact when piped. |
| `skillkit` | Installs an embedded `SKILL.md` and reports `created`, `unchanged`, `outdated` or `foreign`. Never touches a file it didn't write. |
| `status` | Writes OSC 7501 program status reports: idle, working, blocked, done, error. |

### termcolor

```go
var th termcolor.Theme
// replies carries OSC reply bodies your input loop decodes, such as "11;rgb:1e1e/1f1f/2222".
err := termcolor.Wait(ctx, os.Stdout, replies, &th, nil, termcolor.Blue)
muted := termcolor.Mix(th.FG, th.BG, 0.5)
```

`Wait` asks every 250ms for up to 3s, because a new Rex pane may not be
attached to the app when it starts. It returns what arrived by then.

### cliout

| Code | Constant | Meaning |
|---|---|---|
| 0 | `OK` | success |
| 1 | `Failure` | the command ran and failed |
| 2 | `Usage` | invalid command line |
| 3 | `NotFound` | target not found or ambiguous |
| 4 | `Unreachable` | server or dependency unreachable |
| 130 | `Interrupted` | interrupted |

```go
func main() {
	err := run(os.Args[1:])
	os.Exit(cliout.Fail(os.Stderr, "mytool", err)) // prints "mytool: ..." and picks the code
}

return cliout.Errorf(cliout.NotFound, "no agent matches %q", target)
cliout.JSON(os.Stdout, result)
```

### skillkit

```go
//go:embed skills/mytool/SKILL.md
var skillMD []byte

s := skillkit.Skill{Name: "mytool", Tool: "mytool", Content: skillMD}
res, err := s.Install(skillkit.DefaultDir(), dryRun, force)
fmt.Println(s.Describe(res, false)) // "created ~/.claude/skills/mytool/SKILL.md"
```

The installed file gets a marker line after its frontmatter. `Install` only
replaces files carrying its own marker, and `Remove` only deletes them. A
file without the marker is `foreign` and is left alone unless you pass
`force`. Writes go through a temp file and a rename.

### status

```go
status.Report{State: status.Blocked, App: "mytool", Kind: status.Permission,
	Msg: "Delete 3 files?"}.Write(os.Stdout)
status.Report{State: status.Working, App: "mytool", Progress: status.Percent(40)}.Write(os.Stdout)
```

Each report replaces its record completely, so send every field you want to
keep. `msg` and `title` must be one line. Report `done` only for a result the
user hasn't seen. Spec:
https://www.superlogical.com/rex/docs/build/program-status

## Read this, agents

- One folder per package, each with a test file. Run `go test ./...`.
- Add a package only when a tool needs it, and move it here from that tool.
- No package reads the terminal itself. Callers own stdin and pass replies in.
