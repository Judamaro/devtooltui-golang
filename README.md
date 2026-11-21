# devtooltui

Everyday developer utilities (format JSON, decode JWTs, Base64, hashes, regex,
diff…) in **a single binary**: a keyboard-driven TUI and a CLI that composes
with pipes. Works offline, with no telemetry. Inspired by
[DevToys](https://github.com/DevToys-app/DevToys), built for the terminal.

![demo](docs/demo.gif)

## Installation

These put `devtooltui` in the `PATH` with its man pages and shell
completions, so `devtooltui` opens the TUI right away:

```bash
# Homebrew (macOS and Linux)
brew install --cask Judamaro/tap/devtooltui

# Scoop (Windows)
scoop bucket add judamaro https://github.com/Judamaro/scoop-bucket
scoop install devtooltui

# Debian / Ubuntu (use the .rpm or .apk of the release on Fedora or Alpine)
VERSION=1.1.1 ARCH=amd64   # or arm64
curl -LO "https://github.com/Judamaro/devtooltui-golang/releases/download/v$VERSION/devtooltui_${VERSION}_linux_$ARCH.deb"
sudo apt install "./devtooltui_${VERSION}_linux_$ARCH.deb"
```

With Go ≥ 1.26:

```bash
go install github.com/Judamaro/devtooltui-golang/cmd/devtooltui@latest
```

`go install` writes the binary to `$(go env GOPATH)/bin`, usually
`~/go/bin`. If `devtooltui` then says "command not found", add that
directory to your `PATH` once (in `~/.profile`, `~/.bashrc` or `~/.zshrc`)
and open a new terminal:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

Binaries for Linux, macOS and Windows (amd64 and arm64), plus `.deb`, `.rpm`
and `.apk` packages, are available on
[Releases](https://github.com/Judamaro/devtooltui-golang/releases).

Each release has an SPDX SBOM per archive, and `checksums.txt` is signed with
[cosign](https://github.com/sigstore/cosign) (keyless, from the release
workflow). To check a download:

```bash
cosign verify-blob --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github.com/Judamaro/devtooltui-golang/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com checksums.txt
sha256sum --ignore-missing -c checksums.txt
```

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
| `regex` | RegEx | RE2 or regexp2 (.NET: lookaround, backreferences) with highlighted matches and groups |
| `diff` | Diff | By lines (unified or side by side) or by words |
| `html` | HTML entities | Escape and unescape, optionally non-ASCII as `&#N;` |
| `gzip` | GZip | Compress to Base64 and back, with the compression ratio |
| `password` | Password | Random passwords with chosen character sets and their entropy |
| `lorem` | Lorem Ipsum | Words, sentences or paragraphs of placeholder text |
| `case` | Case converter | camelCase, PascalCase, snake_case, kebab-case, CONSTANT, Title… |
| `inspect` | Text inspector | Count characters, words and lines; sort, dedupe or reverse lines |
| `escape` | String escape | Escape and unescape literals for JSON, Go, C#, SQL and shell |
| `json-string` | JSON stringify | JSON to an escaped string (`JSON.stringify`) and back, unwrapping nested strings |
| `jsonpath` | JSONPath | Query JSON with JSONPath; matches as a coloured JSON array |
| `qr` | QR code | Draw a QR code in the terminal; save it as PNG with `Ctrl+S` or `--out` |
| `color` | Color | HEX, RGB, HSL and HSV with a swatch, CSS names and WCAG contrast |
| `cron` | Cron | Explain a cron expression and list its next runs in any time zone |
| `xpath` | XPath | Query XML with XPath 1.0: nodes, attributes, counts and strings |
| `csv-json` | CSV ↔ JSON | CSV to a JSON array (with or without a header row) and back |
| `x509` | X.509 certificate | Subject, SAN, validity, key and fingerprints of PEM or DER certificates |
| `markdown` | Markdown preview | Render Markdown in the terminal: headings, lists, tables, code |
| `json-class` | JSON → classes | Go structs or C# records/classes inferred from a JSON sample |

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

The mouse works too: click a box to focus it, a tool to open it, an option
to select it (on `◀`/`▶` or a toggle, to change it), inside an input to move
the cursor, and an item of the palette to run it; the wheel scrolls the box
under the pointer. While the TUI uses the mouse, hold **Shift** to select
text with the terminal; `mouse = false` in the config turns it off.

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
devtooltui jsonpath --path '$.items[?(@.price < 10)].name' data.json
devtooltui qr --out site.png <<< 'https://example.com'
devtooltui cron --tz Europe/Madrid <<< '30 9 * * 1-5'
devtooltui xpath --expr '//book[price < 10]/title/text()' books.xml
devtooltui csv-json --types data.csv
devtooltui x509 cert.pem
devtooltui json-string <<< '{"a": [1, 2]}'          # → "{\"a\":[1,2]}"
devtooltui json-string decode < stringified.txt
devtooltui markdown README.md
devtooltui json-class --lang csharp --name Order order.json
devtooltui regex --engine regexp2 --pattern '(?<=\$)\d+' prices.txt
pbpaste | devtooltui url decode --copy
```

- Input: positional files (two for `diff` and `jwt`), `-` or stdin.
- Output: stdout. `--json` prints the full result, `--copy` copies it, and
  `--no-color` (or `NO_COLOR`) disables color, which is only used on terminals.
- Exit codes: `0` OK, `1` invalid input (`file:line:col` on stderr), `2`
  usage error, `3` internal error.
- Shell completion: `devtooltui completion bash|zsh|fish|powershell`. Homebrew
  and the `.deb/.rpm/.apk` packages install the completions and the man pages
  (`man devtooltui`, `man devtooltui-json-format`…); the archives carry them
  in `completions/` and `man/`.
- Every tool has a page with its options and examples on the
  [documentation site](docs/index.md).
- From v1.0.0, subcommands, flags, exit codes, the `--json` output and the
  config keys follow SemVer ([ADR 0003](docs/adr/0003-v1-stability.md)).

## Plugins

Any `devtooltui-<name>` executable in the `PATH` that speaks the
[plugin protocol](docs/plugins.md) (JSON over stdin/stdout, in any
language) adds its tools to the TUI and the CLI. `devtooltui plugin list`
shows what was found; [`examples/devtooltui-rot13`](examples/devtooltui-rot13)
is a complete one in Go.

## Configuration

Optional, in `~/.config/devtooltui/config.toml` (the user config directory of
each OS), or the file given by `--config` or `$DEVTOOLTUI_CONFIG`:

```toml
theme = "auto"            # auto | dark | light
detect_on_start = false   # read the clipboard when the TUI opens
debounce_ms = 120
live_max_bytes = 32768    # above it, the input is read-only and runs with F5
mouse = true              # clicks and the wheel (hold Shift to select text)

[plugins]
enabled = true            # look for devtooltui-<name> plugins in the PATH

[history]
enabled = true
remember_options = true   # last options per tool
store_inputs = false      # inputs may hold secrets (JWT, HMAC)

[tools.json-format]       # defaults per tool, same keys as its options
indent = 4
sort_keys = true

[keys]                    # one key, several ("ctrl+y, alt+c") or "none"
copy_output = "alt+c"
quit_outside = "none"
```

Actions for `[keys]`: `next_focus`, `prev_focus`, `toggle_sidebar`, `run`,
`clear_input`, `copy_output`, `paste_input`, `detect_clipboard`,
`open_palette`, `use_output`, `edit_input`, `save_output`, `help`, `quit`,
`quit_outside` and `sidebar_filter`. A key already used by another action is
reported and the default stays.

Unknown keys or invalid values are reported as warnings and ignored.
Command line flags always win over the file.

The TUI remembers the last tool and each tool's options in
`state.json` in the user cache directory (mode 0600). Inputs are only stored
with `store_inputs = true`.

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
