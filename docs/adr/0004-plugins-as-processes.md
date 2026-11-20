# ADR 0004 · Plugins as processes

- Status: accepted (phase 4, 2026-11-20)

## Context

Phase 4 opens devtooltui to third-party tools. The plan offered two
models: external processes found in the `PATH`, like git subcommands,
speaking JSON over stdin/stdout; or WASM modules (wazero, extism), loaded
in-process and sandboxed. ADR 0003 froze the tool contract, so either
model would carry the same `Spec` and `Result` shapes.

## Decision

Plugins are processes: `devtooltui-<name>` executables in the `PATH`,
protocol 1 in [docs/plugins.md](../plugins.md). WASM is not adopted now.

- **Any language, no SDK.** A shell script can be a plugin; WASM needs a
  toolchain per language and an ABI for strings and errors.
- **The contract is already JSON.** `--json` and the frozen `Spec` map 1:1
  to the protocol, so the adapter (`internal/plugin.Tool`) is small.
- **Cancellation and limits come from the OS.** `exec.CommandContext`
  kills a run the TUI no longer needs; a timeout (10 s) and an output cap
  (64 MB) bound a misbehaving plugin.
- **Trust is explicit.** Nothing is downloaded: the user puts the
  executable in the `PATH`, as with git. Plugins cannot replace a built-in
  tool or each other (IDs are first come, first served).

## Consequences

- A run costs a process start (a few ms), fine with the 120 ms debounce.
  Detection cannot afford it (20 ms per detector), so plugins do not take
  part in it until a declarative form exists (phase 4, stage 2).
- Plugins are not sandboxed: they run with the user's permissions. If a
  need appears to run untrusted tools, WASM is the way to isolate them and
  the protocol messages stay the same.
- The adapter is the only `Tool` that does I/O (contract rule 1).
