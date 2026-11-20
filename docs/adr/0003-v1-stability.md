# ADR 0003 · What v1.0.0 keeps stable

- Status: accepted (phase 3 gate, 2026-11-20)

## Context

The roadmap closes phase 3 with v1.0.0 and "the tool contract frozen under
SemVer". All Go code lives in `internal/`, so no one imports it: what users
and scripts depend on is the command line, the configuration and, from
phase 4, the plugin protocol, which will be built on the tool contract.
Before freezing, three inconsistencies were reviewed, because changing them
later would break users.

## Decision

From v1.0.0, changing any of the following needs a new major version;
adding to them (new tools, options, keys, `Meta` entries, choices) does not.

1. **CLI.** Subcommand names are the tool IDs; flags are the option keys
   with `_` → `-` (`sort_keys` → `--sort-keys`); `mode` is also accepted as
   the first argument. Global flags: `--json`, `--copy`, `--no-color`,
   `--max-input`, `--config`, `--clipboard`. Inputs are positional files,
   `-` or stdin, in `Spec.Inputs` order; exporting tools add `--out`.
2. **Exit codes.** `0` OK, `1` invalid input (`file:line:col: message` on
   stderr), `2` usage error, `3` internal error.
3. **`--json` output.** An object with `text`, `lang` (omitted when empty),
   `meta` (a list of `{key, value}`, possibly empty), `diff` (`{op, text}`,
   `op` ∈ `equal`, `insert`, `delete`) and `spans` (`{input, start, end,
   group}`, byte offsets), the last two omitted when empty. Meta keys and
   their order are informative and may grow; scripts should not depend on
   their position.
4. **Configuration.** The keys of `config.toml` documented in the README
   (`theme`, `detect_on_start`, `debounce_ms`, `live_max_bytes`, `[history]`,
   `[tools.<id>]`, `[keys]` and its action names), `$DEVTOOLTUI_CONFIG` and
   the rule that problems are warnings, never errors.
5. **State.** `state.json` keeps its `version` field; a newer format is read
   as an empty history, never as an error.
6. **Tool contract** (`internal/tool`): `Tool`, `Detector` and `Exporter`,
   `Spec`, `Option` (and its four kinds), `Request`, `Result`, `InputError`,
   and the rules in CLAUDE.md. Phase 4 plugins will speak this contract as
   JSON, so its shape is frozen even though the package is internal.

### The three reviewed inconsistencies

- **Query option names stay per tool:** `regex --pattern`, `jsonpath
  --path`, `xpath --expr`. Each is the usual word in its world; one shared
  name (`--query`) would read worse in every tool.
- **`mode` may name a direction.** Rule 2 says reversible tools use
  `encode`/`decode`. Conversions between two formats name the target
  instead: `json-yaml` (`to-yaml`, `to-json`) and `csv-json` (`to-json`,
  `to-csv`). The rule now says so.
- **`cron` uses the local zone by default**, like `timestamp`, so the next
  runs are shown in the user's time. Before v1.0.0 it defaulted to UTC.

## Consequences

- A test or review that changes a flag name, an exit code, a `--json` field
  or a config key is a breaking change and waits for v2.
- New tools and options are minor releases; fixes are patches.
- The goldens, the testscript files and the generated docs act as a record
  of the stable surface: a change in them deserves a second look.
