# devtooltui

Single Go binary with developer utilities (JSON, Base64, JWT, hashes…) in a
TUI (Bubble Tea v2) and a composable CLI. Module:
`github.com/Judamaro/devtooltui-golang`. Binary: `devtooltui`.

## Commands

```bash
make build          # CGO_ENABLED=0 → bin/devtooltui
make test           # go test -race ./...
make lint           # golangci-lint run && govulncheck ./...
make golden-update  # regenerates testdata/**/*.golden (UPDATE_GOLDEN=1)
make bench          # benchmarks (including the textarea spike)
make fuzz           # every Fuzz* target for FUZZTIME (30s; CI runs it too)
```

- Go ≥ 1.26 (required by the `charm.land/*` v2 modules). If `go` is not on the
  PATH: `/usr/local/go/bin`; the lint tools live in `~/go/bin`.
- Before calling anything done, `make test` and `make lint` must pass.
- In a single package, `go test ./internal/tui/ -update` also works.

## Layout

- `cmd/devtooltui`: startup; builds `cli.Deps` and delegates to `cli.Execute`.
- `internal/cli`: cobra. One subcommand per `Spec` (flags generated from the
  options, `sort_keys` → `--sort-keys`; `mode` also as the first argument),
  plus `list`, `detect`, `version` and `completion`. Without arguments and on
  a TTY it opens the TUI. Tests: `testdata/script/*.txtar` (testscript) and
  exit-code tables in `cli_test.go`.
- `internal/detect`: 64 KB sample, 0.5 threshold and a tie below 0.1.
- `internal/clipboard`: atotto + OSC 52 (over SSH or without wl-clipboard/xclip).
- `internal/render`: colouring shared by the TUI and the CLI (Chroma, diff, regex).
- `internal/tool`: the contract (Spec, Option, Request, Result, InputError…).
- `internal/registry`: Spec validation, `Get`/`All`/`ByCategory`, fuzzy
  `Search` and `Detect` (20 ms and `recover` per detector).
- `internal/tools/all.go`: **explicit registration** of every tool. No magic
  `init()`.
- `internal/tools/<category>/<id>.go`: one tool per file.
- `internal/tui`: `app.go` (root model), `keys.go` (central keymap),
  `theme.go` (**all** styles), and the `sidebar/`, `workspace/`,
  `workspace/options/` (panel generated from `Spec.Options`), `statusbar/` and
  `palette/` subpackages. Subpackages expose `Styles` and `KeyMap` structs that
  `theme.go` and `keys.go` fill in.
- `internal/testutil`: golden file helpers (`ReadInput`, `Golden`) and
  `RunCmds` for Bubble Tea commands.
- `testdata/<id>/*.in|*.golden`: per-tool fixtures. `testdata/tui/` keeps the
  views without ANSI.
- `docs/adr/`: architecture decisions.

## Tool contract

```go
type Tool interface {
    Spec() Spec
    Run(ctx context.Context, req Request) (Result, error)
}
type Detector interface { Detect(sample string) float64 } // optional, 0..1
```

Rules (checked by `internal/registry/contract_test.go` over `tools.All()`):

1. **`Run` does no I/O** (no files, network, clipboard or clock unless that is
   its purpose), keeps no global state and honours `ctx`: with an already
   cancelled `ctx` it returns `ctx.Err()`.
2. Reversible tools use the `mode` option (Select) with `encode`/`decode`.
3. Input errors are `*tool.InputError` with line and column (use
   `tool.Position`). Any other error is considered a bug.
4. Binary outputs are shown as Base64 or hex (`output` option).
5. Option `Default` values must match their `Kind` (Toggle→bool,
   Select→string ∈ Choices, Number→int in [Min, Max], String→string).
   `registry.New` validates it.
6. Options are read with `req.Opts.WithDefaults(spec)`.

## Adding a tool

1. Create `internal/tools/<category>/<id>.go` with `Spec`, `Run` and, when it
   applies, `Detect`.
2. Register it in `internal/tools/all.go`.
3. From the first commit: table tests, `testdata/<id>/*.in` with their
   `.golden`, error cases with line and column and, if reversible, a
   round-trip fuzz.

## Current tools

`json-format`, `xml-format`, `base64`, `url`, `jwt`, `timestamp`,
`number-base`, `json-yaml`, `hash`, `uuid`, `regex`, `diff` (registered in
that order in `internal/tools/all.go`; 12 = complete MVP).

- `regex`: the pattern is the `pattern` option (like the CLI `--pattern`
  flag), not an input. `diff` and `jwt` (token + PEM) have two inputs.
- `diff` sets `Result.Lang` to `"diff"` (unified) or `"diff-side"` so the TUI
  knows how to colour it; by words it leaves `Lang` empty and colours `Diff`.
- Fuzz targets that find failures leave cases in
  `internal/tools/<cat>/testdata/fuzz/`: commit them as regressions.
- Clock and randomness: the tools that need them (`jwt`, `timestamp`, `uuid`)
  have optional `Now func() time.Time` and `Rand io.Reader` fields; when nil
  they use `time.Now` and `crypto/rand`. Tests always pin them. Reading them
  does not count as forbidden I/O in `Run`.
- `converters.Relative` ("in 3 h", "2 d ago") is shared.
- `timestamp` embeds `time/tzdata`: IANA zones do not depend on the system.
  Detectors must not touch `time.Local` (loading it took over 20 ms on the
  Windows CI runner).

## TUI

- Tab focus: sidebar → options → input → output (panels that do not apply
  are skipped).
- Options: ↑/↓ changes option, ←/→ changes value, space flips a Toggle.
  Number and String are `textinput`; an out-of-range Number is flagged and
  not applied.
- Async runs: every input change bumps `seq` and schedules a run after the
  debounce (`tui.Options.Debounce`, 120 ms). Starting a run cancels the
  previous `ctx`; a `workspace.ResultMsg` with another `seq` or cancelled is
  dropped. Messages carry `ToolID` and the App routes them. Options, F5, a
  large paste and ctrl+l run without debounce.
- Two inputs sit side by side from 120 terminal columns; below that they
  stack. Tab walks sidebar → options → input A → input B → output.
- Colouring (in the run goroutine, never in `Run`): `Highlighted` with Chroma
  (`theme.go`, 16 ANSI colours), `Matches` with matches in reverse video above
  the list, `DiffView` by prefix/marker. Above 64 KB nothing is coloured
  (`render.MaxBytes`, see `BenchmarkHighlight`).
- The last row is `internal/tui/statusbar`: status on the left, help on the
  right. On error the previous output is dimmed.
- In tests, `testutil.RunCmds` runs the `tea.Cmd`s and delivers the messages
  (dropping those that take over 30 ms, like the cursor blink). Test Apps use
  `Options{Debounce: 0}`.
- `F5` runs (`Live: false` tools and large input mode).
- Overlays (`Ctrl+P` palette, `Ctrl+S` prompt, `?` help) live in
  `internal/tui/palette` and are drawn with `palette.Place` (lipgloss
  compositor). While open they receive every key.
- Shortcuts that must not fire while typing (`q`, `/`, `?`) are checked with
  `App.capturesText()` (input, text option or sidebar filter). `ctrl+l` clears
  the input and leaves large mode.
- TUI behaviour tests use a base64-only registry (`newApp`) so they do not
  depend on the order of `tools.All()`.

## Release

- `.goreleaser.yaml` (GoReleaser v2): linux/darwin/windows × amd64/arm64
  binaries, `checksums.txt`, `.deb/.rpm/.apk` and a cask in
  `Judamaro/homebrew-tap` (needs the `HOMEBREW_TAP_GITHUB_TOKEN` secret).
- `.github/workflows/release.yml` publishes when a `v*` tag is pushed; CI runs
  `goreleaser check` and a snapshot. `make release-dry` runs the snapshot
  locally.
- `docs/demo.gif` is generated by the manual `Demo` workflow from
  `docs/demo.tape`.
- Create tags, publish or create repositories only with explicit confirmation.

## Conventions

- Small commits in Conventional Commits format (`feat(tools): …`,
  `test(tui): …`, `docs: …`). Everything is in English: code comments, UI
  text, tool output, CLI help, tests and docs.
- Do not change the `internal/tool` contract or the folder layout without
  proposing it first.
- Work by phases (see the roadmap): a phase does not start until the previous
  gate is approved.
- TUI: no colours or inline styles outside `theme.go`. Dimensions are
  recomputed on `tea.WindowSizeMsg`. `q` only quits outside text fields.
- Large inputs (ADR 0002, `workspace.DefaultLimits`): above 32 KB or 5,000
  lines the input moves to a read-only viewport; line numbers are hidden above
  16 KB. The `textarea` truncates at 10,000 lines and renders in O(size), so it
  never receives more than that.
- Weak algorithms (MD5, SHA-1) carry `//nolint:gosec` with a reason.
- Config environment variable (phase 2): `DEVTOOLTUI_CONFIG`.
