package tui

import (
	"image/color"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"charm.land/glamour/v2/styles"
	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"

	"github.com/Judamaro/devtooltui-golang/internal/render"
	"github.com/Judamaro/devtooltui-golang/internal/tui/palette"
	"github.com/Judamaro/devtooltui-golang/internal/tui/sidebar"
	"github.com/Judamaro/devtooltui-golang/internal/tui/statusbar"
	"github.com/Judamaro/devtooltui-golang/internal/tui/workspace"
	"github.com/Judamaro/devtooltui-golang/internal/tui/workspace/options"
)

// Theme gathers every TUI style. No other file defines colours or inline
// styles (the colour swatch of render.Swatch is data, not theme).
type Theme struct {
	Dark      bool
	Separator lipgloss.Style
	Palette   palette.Styles
	Status    statusbar.Styles
	Sidebar   sidebar.Styles
	Workspace workspace.Styles
	Help      help.Styles
}

// colors are the ANSI colours a theme is built from.
type colors struct {
	accent, muted, err, ok, warn color.Color
	chroma                       *chroma.Style
	diff                         render.DiffStyles
}

// darkChroma colours JSON, YAML and XML with ANSI colour names, which the
// terminal16 formatter maps to the terminal's palette.
var darkChroma = chroma.MustNewStyle("devtooltui-dark", chroma.StyleEntries{
	chroma.NameTag:         "#ansiblue",      // JSON/YAML keys, XML tags
	chroma.NameAttribute:   "#ansiturquoise", // XML attributes
	chroma.LiteralString:   "#ansigreen",
	chroma.LiteralNumber:   "#ansiyellow",
	chroma.KeywordConstant: "#ansifuchsia", // true, false, null
	chroma.Comment:         "#ansidarkgray",
	chroma.CommentPreproc:  "#ansidarkgray",
	chroma.Punctuation:     "#ansilightgray",
	chroma.NameBuiltin:     "#ansifuchsia",
})

// lightChroma uses the normal (not bright) colours, readable on white.
var lightChroma = chroma.MustNewStyle("devtooltui-light", chroma.StyleEntries{
	chroma.NameTag:         "#ansidarkblue",
	chroma.NameAttribute:   "#ansiteal",
	chroma.LiteralString:   "#ansidarkgreen",
	chroma.LiteralNumber:   "#ansibrown",
	chroma.KeywordConstant: "#ansipurple",
	chroma.Comment:         "#ansidarkgray",
	chroma.CommentPreproc:  "#ansidarkgray",
	chroma.Punctuation:     "#ansidarkgray",
	chroma.NameBuiltin:     "#ansipurple",
})

func paletteFor(dark bool) colors {
	fg := func(c string) lipgloss.Style { return lipgloss.NewStyle().Foreground(lipgloss.Color(c)) }
	if dark {
		return colors{
			accent: lipgloss.Color("12"), muted: lipgloss.Color("8"), err: lipgloss.Color("9"),
			ok: lipgloss.Color("10"), warn: lipgloss.Color("11"), chroma: darkChroma,
			diff: render.DiffStyles{Insert: fg("10"), Delete: fg("9"), Change: fg("11"), Hunk: fg("12")},
		}
	}
	return colors{
		accent: lipgloss.Color("4"), muted: lipgloss.Color("8"), err: lipgloss.Color("1"),
		ok: lipgloss.Color("2"), warn: lipgloss.Color("3"), chroma: lightChroma,
		diff: render.DiffStyles{Insert: fg("2"), Delete: fg("1"), Change: fg("3"), Hunk: fg("4")},
	}
}

// RenderStyles are the dark output colours; the CLI uses them.
func RenderStyles() render.Styles { return RenderStylesFor(true) }

// RenderStylesFor returns the output colours of the dark or light theme.
func RenderStylesFor(dark bool) render.Styles {
	c := paletteFor(dark)
	md := styles.DarkStyleConfig
	if !dark {
		md = styles.LightStyleConfig
	}
	return render.Styles{Chroma: c.chroma, Match: lipgloss.NewStyle().Reverse(true), Diff: c.diff, Markdown: &md}
}

// DarkTheme uses only the 16 ANSI colours.
func DarkTheme() Theme { return ThemeFor(true) }

// LightTheme is for terminals with a light background.
func LightTheme() Theme { return ThemeFor(false) }

// ThemeFor builds the dark or light theme.
func ThemeFor(dark bool) Theme {
	c := paletteFor(dark)
	accent, muted, errC, ok, warn := c.accent, c.muted, c.err, c.ok, c.warn
	return Theme{
		Dark:      dark,
		Separator: lipgloss.NewStyle().Foreground(muted),
		Palette: palette.Styles{
			Box:      lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(accent).Padding(0, 1),
			Title:    lipgloss.NewStyle().Bold(true).Foreground(accent),
			Item:     lipgloss.NewStyle(),
			Selected: lipgloss.NewStyle().Bold(true).Foreground(accent),
			Detail:   lipgloss.NewStyle().Foreground(muted),
			Input:    textinput.DefaultStyles(dark),
		},
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
			Filter:          lipgloss.NewStyle().Foreground(accent),
			More:            lipgloss.NewStyle().Foreground(muted),
		},
		Workspace: workspace.Styles{
			Title:       lipgloss.NewStyle().Bold(true).Foreground(accent),
			Description: lipgloss.NewStyle().Foreground(muted),
			Options: options.Styles{
				Label:   lipgloss.NewStyle().Foreground(muted),
				Value:   lipgloss.NewStyle(),
				Focused: lipgloss.NewStyle().Foreground(accent).Bold(true),
				Invalid: lipgloss.NewStyle().Foreground(errC),
				Input:   textinput.DefaultStyles(dark),
			},
			Section:        lipgloss.NewStyle().Foreground(muted),
			SectionFocused: lipgloss.NewStyle().Foreground(accent),
			OutputStale:    lipgloss.NewStyle().Faint(true),
			ErrorText:      lipgloss.NewStyle().Foreground(errC),
			ErrorMark:      lipgloss.NewStyle().Foreground(errC).Underline(true).Bold(true),
			Render:         RenderStylesFor(dark),
			Textarea:       textarea.DefaultStyles(dark),
		},
		Help: help.DefaultStyles(dark),
	}
}
