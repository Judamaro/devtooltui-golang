package tui

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
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
// styles. The light/auto themes arrive in phase 2.
type Theme struct {
	Separator lipgloss.Style
	Palette   palette.Styles
	Status    statusbar.Styles
	Sidebar   sidebar.Styles
	Workspace workspace.Styles
	Help      help.Styles
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

// RenderStyles are the output colours; the CLI uses them too.
func RenderStyles() render.Styles {
	return render.Styles{
		Chroma: darkChroma,
		Match:  lipgloss.NewStyle().Reverse(true),
		Diff: render.DiffStyles{
			Insert: lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
			Delete: lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
			Change: lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
			Hunk:   lipgloss.NewStyle().Foreground(lipgloss.Color("12")),
		},
	}
}

// DarkTheme uses only the 16 ANSI colours.
func DarkTheme() Theme {
	var (
		accent = lipgloss.Color("12") // bright blue
		muted  = lipgloss.Color("8")
		errC   = lipgloss.Color("9")
		ok     = lipgloss.Color("10")
		warn   = lipgloss.Color("11")
	)
	h := help.New()
	return Theme{
		Separator: lipgloss.NewStyle().Foreground(muted),
		Palette: palette.Styles{
			Box:      lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(accent).Padding(0, 1),
			Title:    lipgloss.NewStyle().Bold(true).Foreground(accent),
			Item:     lipgloss.NewStyle(),
			Selected: lipgloss.NewStyle().Bold(true).Foreground(accent),
			Detail:   lipgloss.NewStyle().Foreground(muted),
			Input:    textinput.DefaultDarkStyles(),
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
			ErrorText:      lipgloss.NewStyle().Foreground(errC),
			ErrorMark:      lipgloss.NewStyle().Foreground(errC).Underline(true).Bold(true),
			Render:         RenderStyles(),
			Textarea:       textarea.DefaultDarkStyles(),
		},
		Help: h.Styles,
	}
}
