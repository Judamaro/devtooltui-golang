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
  `theme.go` (**todos** los estilos), y los subpaquetes `sidebar/` y
  `workspace/`. Los subpaquetes exponen un struct `Styles` que llena `theme.go`.
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
- Entradas grandes: ver ADR 0002 (`live_max_bytes`; el `textarea` trunca a
  10 000 líneas y renderiza en O(tamaño)).
- Variable de entorno de config (fase 2): `DEVTOOLTUI_CONFIG`.
