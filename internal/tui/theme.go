package tui

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"

	"github.com/Judamaro/devtooltui-golang/internal/tui/sidebar"
	"github.com/Judamaro/devtooltui-golang/internal/tui/statusbar"
	"github.com/Judamaro/devtooltui-golang/internal/tui/workspace"
	"github.com/Judamaro/devtooltui-golang/internal/tui/workspace/options"
)

// Theme reúne todos los estilos de la TUI. Ningún otro archivo define
// colores ni estilos en línea. Los temas light/auto llegan en la fase 2.
type Theme struct {
	Separator lipgloss.Style
	Status    statusbar.Styles
	Sidebar   sidebar.Styles
	Workspace workspace.Styles
	Help      help.Styles
}

// darkChroma colorea JSON, YAML y XML con nombres de colores ANSI, que el
// formatter terminal16 traduce a la paleta de la terminal.
var darkChroma = chroma.MustNewStyle("devtooltui-dark", chroma.StyleEntries{
	chroma.NameTag:         "#ansiblue",      // claves JSON/YAML, etiquetas XML
	chroma.NameAttribute:   "#ansiturquoise", // atributos XML
	chroma.LiteralString:   "#ansigreen",
	chroma.LiteralNumber:   "#ansiyellow",
	chroma.KeywordConstant: "#ansifuchsia", // true, false, null
	chroma.Comment:         "#ansidarkgray",
	chroma.CommentPreproc:  "#ansidarkgray",
	chroma.Punctuation:     "#ansilightgray",
	chroma.NameBuiltin:     "#ansifuchsia",
})

// DarkTheme usa solo los 16 colores ANSI.
func DarkTheme() Theme {
	var (
		accent = lipgloss.Color("12") // azul brillante
		muted  = lipgloss.Color("8")
		errC   = lipgloss.Color("9")
		ok     = lipgloss.Color("10")
		warn   = lipgloss.Color("11")
	)
	h := help.New()
	return Theme{
		Separator: lipgloss.NewStyle().Foreground(muted),
		Status: statusbar.Styles{
			OK:      lipgloss.NewStyle().Foreground(ok),
			Error:   lipgloss.NewStyle().Foreground(errC),
			Warning: lipgloss.NewStyle().Foreground(warn),
			Running: lipgloss.NewStyle().Foreground(muted),
			Help:    lipgloss.NewStyle().Foreground(muted),
		},
		Sidebar: sidebar.Styles{
			Category:        lipgloss.NewStyle().Bold(true),
			Item:            lipgloss.NewStyle(),
			Selected:        lipgloss.NewStyle().Foreground(accent).Bold(true),
			SelectedBlurred: lipgloss.NewStyle().Foreground(accent),
		},
		Workspace: workspace.Styles{
			Title:       lipgloss.NewStyle().Bold(true).Foreground(accent),
			Description: lipgloss.NewStyle().Foreground(muted),
			Options: options.Styles{
				Label:   lipgloss.NewStyle().Foreground(muted),
				Value:   lipgloss.NewStyle(),
				Focused: lipgloss.NewStyle().Foreground(accent).Bold(true),
				Invalid: lipgloss.NewStyle().Foreground(errC),
				Input:   textinput.DefaultDarkStyles(),
			},
			Section:        lipgloss.NewStyle().Foreground(muted),
			SectionFocused: lipgloss.NewStyle().Foreground(accent),
			OutputStale:    lipgloss.NewStyle().Faint(true),
			Chroma:         darkChroma,
			Match:          lipgloss.NewStyle().Reverse(true),
			Textarea:       textarea.DefaultDarkStyles(),
		},
		Help: h.Styles,
	}
}
