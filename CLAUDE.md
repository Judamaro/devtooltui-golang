# devtooltui

Binario único en Go con utilidades de desarrollo (JSON, Base64, JWT, hashes…)
en una TUI (Bubble Tea v2) y una CLI componible. Módulo:
`github.com/Judamaro/devtooltui-golang`. Binario: `devtooltui`.

## Comandos

```bash
make build          # CGO_ENABLED=0 → bin/devtooltui
make test           # go test -race ./...
make lint           # golangci-lint run && govulncheck ./...
make golden-update  # regenera testdata/**/*.golden (UPDATE_GOLDEN=1)
make bench          # benchmarks (incluye el spike de textarea)
```

- Go ≥ 1.26 (lo exigen los módulos `charm.land/*` v2). Si `go` no está en el
  PATH: `/usr/local/go/bin`; las herramientas de lint están en `~/go/bin`.
- Antes de dar algo por terminado, `make test` y `make lint` deben pasar.
- En un paquete concreto también vale `go test ./internal/tui/ -update`.

## Estructura

- `cmd/devtooltui`: arranque. Sin argumentos y en una TTY abre la TUI.
- `internal/tool`: el contrato (Spec, Option, Request, Result, InputError…).
- `internal/registry`: validación de Specs, `Get`/`All`/`ByCategory`,
  `Search` (difusa) y `Detect` (20 ms y `recover` por detector).
- `internal/tools/all.go`: **registro explícito** de todas las herramientas.
  Sin `init()` mágicos.
- `internal/tools/<categoría>/<id>.go`: una herramienta por archivo.
- `internal/tui`: `app.go` (modelo raíz), `keys.go` (keymap central),
  `theme.go` (**todos** los estilos), y los subpaquetes `sidebar/`,
  `workspace/` y `workspace/options/` (panel generado desde `Spec.Options`).
  Los subpaquetes exponen structs `Styles` y `KeyMap` que llenan `theme.go`
  y `keys.go`.
- `internal/testutil`: helpers de golden files (`ReadInput`, `Golden`).
- `testdata/<id>/*.in|*.golden`: fixtures por herramienta. `testdata/tui/`
  guarda las vistas sin ANSI.
- `docs/adr/`: decisiones de arquitectura.

## Contrato Tool

```go
type Tool interface {
    Spec() Spec
    Run(ctx context.Context, req Request) (Result, error)
}
type Detector interface { Detect(sample string) float64 } // opcional, 0..1
```

Reglas (las verifica `internal/registry/contract_test.go` sobre `tools.All()`):

1. **`Run` no hace I/O** (ni archivos, ni red, ni portapapeles, ni reloj
   salvo que sea su propósito), no guarda estado global y respeta `ctx`:
   con `ctx` ya cancelado devuelve `ctx.Err()`.
2. Las herramientas reversibles usan la opción `mode` (Select) con
   `encode`/`decode`.
3. Los errores de entrada son `*tool.InputError` con línea y columna (usa
   `tool.Position`). Cualquier otro error se considera un bug.
4. Las salidas binarias se representan en Base64 o hex (opción `output`).
5. Los `Default` de las opciones deben coincidir con su `Kind` (Toggle→bool,
   Select→string ∈ Choices, Number→int en [Min, Max], String→string).
   `registry.New` lo valida.
6. Las opciones se leen con `req.Opts.WithDefaults(spec)`.

## Agregar una herramienta

1. Crear `internal/tools/<categoría>/<id>.go` con `Spec`, `Run` y, si aplica,
   `Detect`.
2. Registrarla en `internal/tools/all.go`.
3. Desde el primer commit: tests de tabla, `testdata/<id>/*.in` con sus
   `.golden`, casos de error con línea y columna y, si es reversible, un
   fuzz de ida y vuelta.

## Herramientas actuales

`json-format`, `xml-format`, `base64`, `url`, `jwt`, `timestamp`,
`number-base`, `json-yaml`, `hash`, `uuid`, `regex`, `diff` (registradas en
ese orden en `internal/tools/all.go`; 12 = MVP completo).

- `regex`: el patrón es la opción `pattern` (como el flag `--pattern` de la
  CLI), no una entrada. `diff` y `jwt` (token + PEM) tienen dos entradas.
- `diff` usa `Result.Lang` = `"diff"` (unificado) o `"diff-side"` para que la
  TUI sepa cómo colorear; por palabras deja `Lang` vacío y colorea `Diff`.
- Los fuzz que encuentran fallos dejan casos en
  `internal/tools/<cat>/testdata/fuzz/`: se commitean como regresión.

- Reloj y aleatoriedad: las herramientas que los necesitan (`jwt`,
  `timestamp`, `uuid`) tienen campos opcionales `Now func() time.Time` y
  `Rand io.Reader`; con nil usan `time.Now` y `crypto/rand`. Los tests
  siempre los fijan. Leerlos no cuenta como I/O prohibido en `Run`.
- `converters.Relative` ("en 3 h", "hace 2 d") es compartido.
- `timestamp` embebe `time/tzdata`: las zonas IANA no dependen del sistema.

## TUI

- Foco con Tab: sidebar → opciones → entrada → salida (se saltan los paneles
  que no aplican).
- Opciones: ↑/↓ cambia de opción, ←/→ cambia el valor, espacio invierte un
  Toggle. Number y String son `textinput`; un Number fuera de rango se marca
  y no se aplica.
- Ejecución asíncrona: cada cambio de entrada sube `seq` y programa la
  ejecución tras el debounce (`tui.Options.Debounce`, 120 ms). Lanzar una
  ejecución cancela el `ctx` anterior; `workspace.ResultMsg` con otra `seq`
  o cancelado se descarta. Los mensajes llevan `ToolID` y la App los enruta.
  Opciones, F5, pegado grande y ctrl+l ejecutan sin debounce.
- Dos entradas van lado a lado desde 120 columnas de terminal; si no, apiladas.
  Tab recorre sidebar → opciones → entrada A → entrada B → salida.
- Coloreado (en la goroutine de ejecución, nunca en `Run`): `Highlighted` con
  Chroma (`theme.go`, 16 colores ANSI), `Matches` con las coincidencias en
  video inverso sobre la lista, `DiffView` por prefijo/marcador. Por encima
  de 64 KB no se colorea (`HighlightMaxBytes`, ver `BenchmarkHighlight`).
- La última fila es `internal/tui/statusbar`: estado a la izquierda, ayuda a
  la derecha. Con error, la salida anterior se atenúa.
- En tests, `testutil.RunCmds` ejecuta los `tea.Cmd` y entrega los mensajes
  (descarta los que tardan más de 30 ms, como el parpadeo del cursor). Las
  Apps de test usan `Options{Debounce: 0}`.
- `F5` ejecuta (herramientas `Live: false` y modo de entrada grande).
  `ctrl+l` limpia la entrada y sale del modo grande.
- Los tests de comportamiento de la TUI usan un registro solo con `base64`
  (`newApp`) para no depender del orden de `tools.All()`.

## Convenciones

- Commits pequeños en formato Conventional Commits (`feat(tools): …`,
  `test(tui): …`, `docs: …`), en inglés. Los comentarios y textos de la UI van
  en español.
- No cambiar el contrato de `internal/tool` ni la estructura de carpetas sin
  proponerlo antes.
- Trabajo por fases (ver el roadmap): no se empieza una fase sin aprobar el
  gate de la anterior.
- TUI: ningún color ni estilo en línea fuera de `theme.go`. Las dimensiones se
  recalculan en `tea.WindowSizeMsg`. `q` solo sale fuera de los campos de
  texto.
- Entradas grandes (ADR 0002, `workspace.DefaultLimits`): por encima de
  32 KB o 5 000 líneas la entrada pasa a un viewport de solo lectura; los
  números de línea se ocultan por encima de 16 KB. El `textarea` trunca a
  10 000 líneas y renderiza en O(tamaño), así que nunca recibe más que eso.
- Los algoritmos débiles (MD5, SHA-1) llevan `//nolint:gosec` con motivo.
- Variable de entorno de config (fase 2): `DEVTOOLTUI_CONFIG`.
