# ADR 0001 · Bubble Tea v2

- Status: accepted (phase 0, 2026-11-19)

## Context

The plan left open the choice between Bubble Tea v1 and v2. v2 changes the
imports, the keyboard messages and rendering, and migrating later is costly.

## Decision

Use the v2 stack from the start:

| Module | Version at decision time |
| --- | --- |
| `charm.land/bubbletea/v2` | v2.1.2 (stable since v2.0.0, Feb 2026) |
| `charm.land/bubbles/v2` | v2.2.1 |
| `charm.land/lipgloss/v2` | v2.1.1 |
| `github.com/charmbracelet/x/exp/teatest/v2` | pseudo-version (no tags; it is `x/exp`) |

## Consequences

- `View()` returns `tea.View`; keys arrive as `tea.KeyPressMsg` and a paste as
  a single `tea.PasteMsg` (native bracketed paste).
- Charm's v2 modules require **Go ≥ 1.26**, so `go.mod` declares `go 1.26`
  (the plan said ≥ 1.25).
- `teatest/v2` has no tagged versions: it is pinned by pseudo-version and
  reviewed on upgrades.
- To verify in their phase: Glamour with Lip Gloss v2 (phase 3) and the new
  signature of `lipgloss.HasDarkBackground` (phase 2).
