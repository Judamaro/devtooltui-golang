# ADR 0002 · Umbral de edición en vivo (`live_max_bytes`)

- Estado: propuesta (fase 0, 2026-11-19), pendiente de aprobar en el gate

## Contexto

El objetivo es menos de 50 ms de latencia percibida con entradas de hasta 1 MB.
El spike (`internal/tui/workspace/textarea_bench_test.go`) midió
`bubbles/v2/textarea` v2.2.1 con JSON indentado (muchas líneas) y minificado
(una línea), en un textarea de 80×20.

Máquina: AMD Ryzen 7 7700 (16 hilos), Go 1.27.3. Comando:

```bash
go test -run Spike -v -bench . -benchtime 5x ./internal/tui/workspace/
```

## Resultados

**1. El textarea trunca sin avisar a 10 000 líneas** (constante `maxLines`
interna, no configurable):

| Entrada | Líneas | Conserva |
| --- | --- | --- |
| indentado 100 KB | 7 916 | 100 % |
| indentado 500 KB | 39 320 | 25 % |
| indentado 1 MB | 80 436 | 12 % |
| minificado 1–2 MB | 1 | 100 % |

**2. Una tecla + `View()` cuesta O(tamaño total)**: `textarea.view()` estiliza
todas las líneas en cada frame, no solo las visibles.

| Tamaño | Indentado (con nº de línea) | Indentado (sin nº) | Minificado |
| --- | --- | --- | --- |
| 8 KB | 11.7 ms | 8.4 ms | 6.0 ms |
| 16 KB | 22.8 ms | 17.0 ms | 11.7 ms |
| 32 KB | **44.8 ms** | 33.3 ms | 23.2 ms |
| 64 KB | 89.6 ms | 66.8 ms | 45.5 ms |
| 1 MB | 173 ms (truncado) | — | 776 ms |

Pegar 1 MB (`PasteMsg` + `View`): 181 ms indentado (truncado) y 683 ms
minificado.

**3. Un `viewport` de solo lectura es fluido**: cargar 1 MB (`SetContent`)
tarda 13–30 ms una sola vez, y cada frame posterior (scroll + `View`) tarda
0.03 ms indentado y 10 ms minificado.

## Decisión propuesta

- `live_max_bytes = 32768` (32 KB) por defecto, en lugar del 1 MB del plan.
  Además, un límite de **5 000 líneas** para no acercarse al tope de 10 000.
- Por debajo de ambos umbrales, la entrada se edita en el `textarea` y se
  recalcula en vivo con debounce.
- Por encima, se usa la mitigación que ya prevé el plan: la entrada se muestra
  en un `viewport` de solo lectura, se recalcula con `F5` (o una vez al pegar)
  y se edita con `$EDITOR` (`Ctrl+E`). La statusbar avisa del cambio de modo.
- Se ocultan los números de línea en entradas de más de 16 KB (ahorran ~25 %).

## Alternativa para más adelante

Escribir un editor propio en `internal/tui/editor` que renderice solo las
líneas visibles (O(alto de pantalla) por frame) y no tenga tope de líneas. Con
eso se cumpliría el objetivo de 1 MB editable en vivo. Es trabajo de 1–2
semanas y cambia el stack (deja de usar `bubbles/textarea`), así que se
propone como decisión aparte para la fase 2 si 32 KB resulta poco en la
práctica.
