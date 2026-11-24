# ADR 0005 · TUI: TEA conventions

- Status: accepted (2026-11-24)

## Context

Bubble Tea is The Elm Architecture (TEA) in Go: a model, `Update(msg)`
that returns the next state and a command, `View` that draws the state,
and effects as `tea.Cmd`. The TUI was built on it from the start, but the
mouse, export and editor features added deviations:

- `Update` did I/O: it wrote the saved file and the editor's temp file,
  and called `Export` on the UI goroutine. A plugin export could freeze
  the TUI for up to 10 s.
- `View` had side effects: the palette and the prompt stored their
  geometry for clicks while drawing.
- Components had four `Update` shapes. Examples: the sidebar by value,
  and the options panel returning a `changed` flag.
- The help, palette and prompt were routed with the same
  `if showHelp / prompt != nil / palette != nil` chain in three places:
  keys, clicks and leftover messages.

A rewrite in "pure" Elm style (immutable models copied on every message)
was rejected. Bubble Tea uses pointers idiomatically, and copying the
textarea on every key costs much and adds nothing.

## Decision

These rules are kept in `internal/tui` and its subpackages.

1. **`Update` does no I/O.** Files, processes, the clipboard and exports
   run in a `tea.Cmd`, which answers a typed message:
   - `exportedMsg`, `savedMsg`, `editorFileMsg`, `editorTextMsg`;
   - `clipReadMsg`;
   - `workspace.ResultMsg`.

   `Update` only updates the state from that message.
2. **`View` does not mutate.**
   - Sizes come from `SetSize`, called on `tea.WindowSizeMsg` and when a
     component opens.
   - Hit tests reuse the same pure layout functions that draw, for
     example `palette.box()` and `options.layout()`.
3. **Components share one shape**, the `component` interface in
   `component.go`:
   ```go
   type component interface {
       Update(msg tea.Msg) tea.Cmd
       View() string
       SetSize(w, h int)
   }
   ```
   - They are pointers.
   - A child reports to its parent with a typed message, never with an
     extra return value. Examples are `sidebar.SelectedMsg` and
     `options.ChangedMsg{ToolID}`.
   - Messages that belong to one tool carry `ToolID`, and the App routes
     them to that tool's workspace. Examples are `ResultMsg`,
     `DebounceMsg` and `ChangedMsg`.
   - `Focus`, `Blur` and `SetStyles` are optional and keep their names.
4. **Overlays are a stack.** The help, the palette and the prompt
   implement `overlay` (`overlay.go`):
   - `Update`, `SetSize` and `Overlay(base)`, which composes over the
     screen instead of `View`;
   - `Contains`, plus `Click` for a click inside the overlay.

   The App keeps `overlays []overlay`:
   - the top one gets keys, clicks and leftover messages, such as the
     cursor blink;
   - a click outside it pops it, and so does `palette.ClosedMsg`;
   - a new overlay implements the interface and does not touch
     `App.Update`.

## Consequences

- An export or a save never blocks the UI. Tests run the commands with
  `testutil.RunCmds`, which drops commands slower than 30 ms (ticks). A
  test whose command does real work waits for it with `sendSlow`.
- The interfaces are asserted at compile time, so a component that
  drifts from the shape does not build.
- `options.Model` is a panel inside the workspace's tool box, not a
  section, so it has `Lines(width)` instead of `View`/`SetSize`. It still
  follows rule 3 for messages.
- Nothing visible changed: the TUI goldens were not regenerated.
