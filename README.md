# devtooltui

Everyday developer utilities (format JSON, decode JWTs, Base64, hashes, regex,
diff…) in **a single binary**: a keyboard-driven TUI and a CLI that composes
with pipes. Works offline, with no telemetry. Inspired by
[DevToys](https://github.com/DevToys-app/DevToys), built for the terminal.

> The interface (TUI labels, help and messages) is currently in Spanish.

![demo](docs/demo.gif)

## Installation

```bash
# Homebrew (macOS and Linux)
brew install --cask Judamaro/tap/devtooltui

# Go ≥ 1.26
go install github.com/Judamaro/devtooltui-golang/cmd/devtooltui@latest
```

Binaries for Linux, macOS and Windows (amd64 and arm64), plus `.deb`, `.rpm`
and `.apk` packages, are available on
[Releases](https://github.com/Judamaro/devtooltui-golang/releases).

On Linux, clipboard support needs `wl-clipboard` (Wayland) or `xclip`/`xsel`
(X11). Without them, and over SSH, copying falls back to OSC 52.

## Tools

| ID | Tool | What it does |
| --- | --- | --- |
| `json-format` | JSON | Indent, minify and sort keys; errors with line and column |
| `xml-format` | XML | Indent or minify, keeping comments, CDATA and DOCTYPE |
| `base64` | Base64 | Standard or URL-safe, with or without padding; binary shown as hex |
| `url` | URL | Encode/decode (query or path) and break a URL into its parts |
| `jwt` | JWT | Header, payload and dates; verifies HS* with a secret and RS/PS/ES/EdDSA with a PEM key |
| `timestamp` | Timestamp | Unix (s/ms/µs/ns) ↔ ISO 8601/RFC 1123, in any IANA time zone |
| `number-base` | Number bases | Binary, octal, decimal and hex of any size |
| `json-yaml` | JSON ↔ YAML | Keeps key order; supports anchors and merge keys |
| `hash` | Hash | MD5, SHA-1, SHA-256, SHA-512 and HMAC |
| `uuid` | UUID / ULID | Generates v4, v7 or ULID; inspects version and timestamp |
| `regex` | RegEx | RE2 with highlighted matches and groups |
| `diff` | Diff | By lines (unified or side by side) or by words |

## TUI

```bash
devtooltui              # open the TUI
devtooltui --clipboard  # open the tool that matches the clipboard
```

| Key | Action |
| --- | --- |
| `Tab` / `Shift+Tab` | Cycle focus: sidebar, options, inputs, output |
| `Ctrl+P` | Command palette: search for a tool or action |
| `/` | Filter the sidebar |
| `Ctrl+Y` / `Ctrl+O` | Copy the output / paste into the input |
| `Ctrl+D` | Detect the clipboard and jump to its tool |
| `Ctrl+R` | Use the output as input (chain tools) |
| `Ctrl+E` | Edit the input in `$EDITOR` |
| `Ctrl+S` | Save the output to a file |
| `Ctrl+L` | Clear the input |
| `Ctrl+B` | Show or hide the sidebar |
| `F5` | Run (large inputs or non-live tools) |
| `?` | Show all shortcuts |
| `q` / `Ctrl+C` | Quit (`q` only outside text fields) |

Inputs over 32 KB switch to read-only mode (rerun with `F5`, edit with
`Ctrl+E`); see [ADR 0002](docs/adr/0002-live-max-bytes.md).

## CLI

Every tool is a subcommand, and its options are flags.

```bash
devtooltui list [--json]
devtooltui detect token.txt                 # → jwt (0.98)
cat data.json | devtooltui json-format --sort-keys --indent 4
devtooltui base64 decode <<< 'aG9sYQ=='
devtooltui jwt --secret "$KEY" < token.txt
devtooltui jwt token.txt public.pem
devtooltui hash --algo sha256 file.bin
devtooltui diff a.txt b.txt
devtooltui regex --pattern '\d{4}-\d{2}' log.txt
devtooltui uuid --version v7 --count 5
pbpaste | devtooltui url decode --copy
```

- Input: positional files (two for `diff` and `jwt`), `-` or stdin.
- Output: stdout. `--json` prints the full result, `--copy` copies it, and
  `--no-color` (or `NO_COLOR`) disables color, which is only used on terminals.
- Exit codes: `0` OK, `1` invalid input (`file:line:col` on stderr), `2`
  usage error, `3` internal error.
- Shell completion: `devtooltui completion bash|zsh|fish|powershell`.

## Development

```bash
make build   # bin/devtooltui
make test    # go test -race ./...
make lint    # golangci-lint + govulncheck
```

Project conventions (the `Tool` contract, tests and golden files) are in
[CLAUDE.md](CLAUDE.md), and design decisions in [docs/adr](docs/adr).

## License

[MIT](LICENSE). Inspired by [DevToys](https://github.com/DevToys-app/DevToys)
(MIT); it shares no code with it.
