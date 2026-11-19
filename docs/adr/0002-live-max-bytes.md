# ADR 0002 · Live editing threshold (`live_max_bytes`)

- Status: accepted (phase 0 gate, 2026-11-19)

## Context

The goal is under 50 ms of perceived latency with inputs of up to 1 MB. The
spike (`internal/tui/workspace/textarea_bench_test.go`) measured
`bubbles/v2/textarea` v2.2.1 with indented JSON (many lines) and minified JSON
(one line), in an 80×20 textarea.

Machine: AMD Ryzen 7 7700 (16 threads), Go 1.27.3. Command:

```bash
go test -run Spike -v -bench . -benchtime 5x ./internal/tui/workspace/
```

## Results

**1. The textarea silently truncates at 10,000 lines** (internal `maxLines`
constant, not configurable):

| Input | Lines | Kept |
| --- | --- | --- |
| indented 100 KB | 7,916 | 100% |
| indented 500 KB | 39,320 | 25% |
| indented 1 MB | 80,436 | 12% |
| minified 1–2 MB | 1 | 100% |

**2. One key + `View()` costs O(total size)**: `textarea.view()` styles every
line on each frame, not only the visible ones.

| Size | Indented (line numbers) | Indented (no numbers) | Minified |
| --- | --- | --- | --- |
| 8 KB | 11.7 ms | 8.4 ms | 6.0 ms |
| 16 KB | 22.8 ms | 17.0 ms | 11.7 ms |
| 32 KB | **44.8 ms** | 33.3 ms | 23.2 ms |
| 64 KB | 89.6 ms | 66.8 ms | 45.5 ms |
| 1 MB | 173 ms (truncated) | — | 776 ms |

Pasting 1 MB (`PasteMsg` + `View`): 181 ms indented (truncated) and 683 ms
minified.

**3. A read-only `viewport` is smooth**: loading 1 MB (`SetContent`) takes
13–30 ms once, and each later frame (scroll + `View`) takes 0.03 ms indented
and 10 ms minified.

## Decision

- `live_max_bytes = 32768` (32 KB) by default, instead of the plan's 1 MB, plus
  a **5,000-line** limit to stay away from the 10,000 cap.
- Below both thresholds the input is edited in the `textarea` and recomputed
  live with debounce.
- Above them, the plan's mitigation applies: the input is shown in a read-only
  `viewport`, recomputed with `F5` (or once on paste) and edited with
  `$EDITOR` (`Ctrl+E`). The statusbar announces the mode change.
- Line numbers are hidden for inputs over 16 KB (they cost ~25%).

## Alternative for later

Write our own editor in `internal/tui/editor` that renders only the visible
lines (O(screen height) per frame) with no line cap. That would meet the
1 MB live-editing goal. It is 1–2 weeks of work and changes the stack (it
drops `bubbles/textarea`), so it is proposed as a separate decision for
phase 2 if 32 KB turns out to be too little in practice.
