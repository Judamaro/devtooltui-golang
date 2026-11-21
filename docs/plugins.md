# Plugins

A plugin adds tools to devtooltui without rebuilding it. It is any
executable named `devtooltui-<name>` in the `PATH` (`devtooltui-<name>.exe`
on Windows), written in any language, that speaks JSON over stdin and
stdout. Its tools appear in the TUI and as CLI subcommands, with flags from
their options, exactly like the built-in ones.

```bash
devtooltui plugin list          # plugins found, their tools and their problems
devtooltui plugin list --json
```

devtooltui never downloads plugins: it runs what you put in your `PATH`,
with the same trust as a git subcommand. See
[ADR 0004](adr/0004-plugins-as-processes.md) for why processes and not WASM.
`[plugins] enabled = false` in the [config](https://github.com/Judamaro/devtooltui-golang#configuration)
turns them off.

A complete example in Go, with tests, is
[`examples/devtooltui-rot13`](https://github.com/Judamaro/devtooltui-golang/tree/main/examples/devtooltui-rot13):

```bash
go build -o ~/bin/devtooltui-rot13 ./examples/devtooltui-rot13
echo 'Hello' | devtooltui rot13 --variant rot47
```

## Protocol 1

devtooltui runs the plugin with one argument and sets
`DEVTOOLTUI_PROTOCOL=1` in its environment.

### `devtooltui-<name> spec`

Prints the tools it offers (5 s at most):

```json
{
  "protocol": 1,
  "tools": [
    {
      "id": "shout",
      "name": "Shout",
      "category": "Text",
      "description": "Shout the text",
      "keywords": ["upper", "loud"],
      "inputs": [{"key": "input", "label": "Text", "multiline": true}],
      "options": [
        {"key": "marks", "label": "Marks", "kind": "number", "min": 1, "max": 5, "default": 1},
        {"key": "style", "label": "Style", "kind": "select", "choices": ["plain", "bold"], "default": "plain"}
      ],
      "output": "plain",
      "live": true
    }
  ]
}
```

- `id`: lowercase kebab-case, unique. An ID already taken by a built-in tool
  or by an earlier plugin in the `PATH` is skipped and reported by
  `plugin list`.
- `category`: `Formatters`, `Encoders`, `Converters`, `Generators`, `Testers`
  or `Text`; anything else goes to `Plugins`.
- `inputs`: 0, 1 or 2. With two, the CLI takes two file arguments.
- `options[].kind`: `toggle` (bool default), `select` (default among
  `choices`), `number` (integer default in `[min, max]`) or `string`. The
  key `mode` with a select is also accepted as the first CLI argument.
- `output`: `plain` (default), `highlighted` (coloured by `result.lang`),
  `diff`, `matches` or `image`.
- `live`: `false` runs only with F5 in the TUI. Default `true`.
- `detect` (optional): `{"regex": "^-----BEGIN FOO", "score": 0.8}`. Smart
  detection (`devtooltui detect`, `Ctrl+D`, `--clipboard`) matches the RE2
  regex against the sample itself and scores `score` (0 < score ≤ 1) on a
  match: the plugin is not run, since a detector has 20 ms.
- `export` (optional): `true` when the tool answers `export` (below).

devtooltui keeps the answer in `<user cache dir>/devtooltui/plugins.json`
and asks again only when the executable changes (size or modification
time), so `spec` should not depend on anything else.

### `devtooltui-<name> run`

Reads one request from stdin:

```json
{"protocol": 1, "tool": "shout", "inputs": {"input": "hello"}, "options": {"marks": 2, "style": "plain"}}
```

Every option is present, with its default when the user did not set it.
It prints exactly one of:

```json
{"result": {"text": "HELLO!!", "lang": "", "meta": [{"key": "Length", "value": "5"}]}}
```

```json
{"error": {"input": "input", "line": 1, "col": 1, "message": "nothing to shout"}}
```

- `result` has the shape of the CLI's `--json`: `text`, and optionally
  `lang`, `meta` (shown in the status bar), `diff` (`{op, text}`, `op` ∈
  `equal`, `insert`, `delete`) and `spans` (`{input, start, end, group}`,
  byte offsets inside that input).
- `error` is a problem with the input: the TUI points at the line and
  column, and the CLI exits with code 1 and `file:line:col: message`.
- Anything else (invalid JSON, both or neither key, a non-zero exit, more
  than 64 MB of output, more than 10 s) is an internal error. Write
  diagnostics to stderr: devtooltui shows its last lines.

### `devtooltui-<name> export` (optional)

For tools that declare `"export": true`: a binary form of the result, which
`Ctrl+S` saves when the path has its extension and the CLI writes with
`--out FILE`. It reads the same request as `run` and prints one of:

```json
{"export": {"data": "iVBORw0KGgo…", "ext": ".png"}}
```

```json
{"error": {"input": "input", "line": 1, "col": 1, "message": "nothing to export"}}
```

`data` is Base64; `ext` starts with a dot.

### Lifetime

The process gets a new request per run. When the user keeps typing,
devtooltui kills the run that is no longer needed, so `run` should be
quick and must not keep state between runs.
