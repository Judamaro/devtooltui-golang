# ADR 0001 · Bubble Tea v2

- Estado: aceptada (fase 0, 2026-11-19)

## Contexto

El plan dejaba abierta la elección entre Bubble Tea v1 y v2. v2 cambia los
imports, los mensajes de teclado y el render, y migrar después cuesta.

## Decisión

Usar el stack v2 desde el inicio:

| Módulo | Versión al decidir |
| --- | --- |
| `charm.land/bubbletea/v2` | v2.1.2 (estable desde v2.0.0, feb 2026) |
| `charm.land/bubbles/v2` | v2.2.1 |
| `charm.land/lipgloss/v2` | v2.1.1 |
| `github.com/charmbracelet/x/exp/teatest/v2` | pseudo-versión (sin tags; es `x/exp`) |

## Consecuencias

- `View()` devuelve `tea.View`; las teclas llegan como `tea.KeyPressMsg` y el
  pegado como un único `tea.PasteMsg` (bracketed paste nativo).
- Los módulos de Charm v2 exigen **Go ≥ 1.26**, así que `go.mod` declara
  `go 1.26` (el plan decía ≥ 1.25).
- `teatest/v2` no tiene versiones etiquetadas: se fija por pseudo-versión y se
  revisa al actualizar.
- Pendiente de verificar en su fase: Glamour con Lip Gloss v2 (fase 3) y la
  nueva firma de `lipgloss.HasDarkBackground` (fase 2).
