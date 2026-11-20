package tui

import (
	"image/color"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/Judamaro/devtooltui-golang/internal/history"
	"github.com/Judamaro/devtooltui-golang/internal/registry"
	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools"
	"github.com/Judamaro/devtooltui-golang/internal/tools/converters"
	"github.com/Judamaro/devtooltui-golang/internal/tools/encoders"
	"github.com/Judamaro/devtooltui-golang/internal/tools/formatters"
	"github.com/Judamaro/devtooltui-golang/internal/tools/testers"
	"github.com/Judamaro/devtooltui-golang/internal/tools/text"
	"github.com/Judamaro/devtooltui-golang/internal/tui/workspace"
)

// newApp builds an App with base64 only, so behaviour tests do not depend
// on the order of the full registry.
func newApp(t *testing.T, w, h int) *App {
	t.Helper()
	return newAppWith(t, w, h, encoders.Base64{})
}

func newAppWith(t *testing.T, w, h int, ts ...tool.Tool) *App {
	t.Helper()
	reg, err := registry.New(ts...)
	if err != nil {
		t.Fatal(err)
	}
	a := New(reg, Options{Debounce: 0})
	send(a, tea.WindowSizeMsg{Width: w, Height: h})
	testutil.RunCmds(a.Init(), func(msg tea.Msg) tea.Cmd { _, c := a.Update(msg); return c })
	return a
}

// send hands msg to the App and runs the commands it returns, so async
// results are already applied on return.
func send(a *App, msg tea.Msg) {
	_, cmd := a.Update(msg)
	testutil.RunCmds(cmd, func(m tea.Msg) tea.Cmd {
		if _, ok := m.(tea.QuitMsg); ok {
			return nil
		}
		_, c := a.Update(m)
		return c
	})
}

func press(a *App, keys ...string) {
	for _, k := range keys {
		var msg tea.KeyPressMsg
		switch k {
		case "tab":
			msg = tea.KeyPressMsg{Code: tea.KeyTab}
		case "shift+tab":
			msg = tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
		case "ctrl+b":
			msg = tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl}
		case "right":
			msg = tea.KeyPressMsg{Code: tea.KeyRight}
		case "left":
			msg = tea.KeyPressMsg{Code: tea.KeyLeft}
		case "down":
			msg = tea.KeyPressMsg{Code: tea.KeyDown}
		case "up":
			msg = tea.KeyPressMsg{Code: tea.KeyUp}
		case "space":
			msg = tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
		default:
			for _, r := range k {
				send(a, tea.KeyPressMsg{Code: r, Text: string(r)})
			}
			continue
		}
		send(a, msg)
	}
}

var elapsedRe = regexp.MustCompile(`✓ [0-9.]+ (µs|ms|s)`)

// plainView strips ANSI codes and masks the run time so the goldens are
// deterministic.
func plainView(a *App) string {
	lines := strings.Split(ansi.Strip(a.View().Content), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(elapsedRe.ReplaceAllString(l, "✓ 0 µs"), " ")
	}
	// The statusbar padding depends on the width of the masked time.
	last := len(lines) - 1
	lines[last] = spacesRe.ReplaceAllString(lines[last], "  ")
	return strings.Join(lines, "\n")
}

var spacesRe = regexp.MustCompile(` {2,}`)

func TestViewGolden(t *testing.T) {
	tests := []struct {
		name string
		w, h int
		do   func(a *App)
	}{
		{"start-100x30", 100, 30, func(*App) {}},
		{"all-tools-100x30", 100, 30, nil},
		{"base64-typed-100x30", 100, 30, func(a *App) { press(a, "tab", "tab", "hola") }},
		{"options-focused-100x30", 100, 30, func(a *App) { press(a, "tab", "right") }},
		{"large-input-110x30", 110, 30, func(a *App) {
			press(a, "tab", "tab")
			send(a, tea.PasteMsg{Content: strings.Repeat("hello world\n", 3500)})
		}},
		{"narrow-80x20", 80, 20, func(a *App) { press(a, "hola") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var a *App
			if tt.do == nil {
				a = newAppWith(t, tt.w, tt.h, tools.All()...)
			} else {
				a = newApp(t, tt.w, tt.h)
				tt.do(a)
			}
			got := plainView(a)
			raw := ansi.Strip(a.View().Content)
			if n := strings.Count(raw, "\n") + 1; n != tt.h {
				t.Errorf("the view has %d lines, want %d", n, tt.h)
			}
			for i, line := range strings.Split(raw, "\n") {
				if w := ansi.StringWidth(line); w > tt.w {
					t.Errorf("line %d is %d columns wide, maximum %d", i+1, w, tt.w)
				}
			}
			testutil.Golden(t, "tui", tt.name, got)
		})
	}
}

func TestFocusRotation(t *testing.T) {
	tests := []struct {
		name string
		w    int
		keys []string
		want focus
	}{
		{"starts in the sidebar", 100, nil, focusSidebar},
		{"tab to options", 100, []string{"tab"}, focusOptions},
		{"tab tab to input", 100, []string{"tab", "tab"}, focusInput},
		{"three tabs to output", 100, []string{"tab", "tab", "tab"}, focusOutput},
		{"full round", 100, []string{"tab", "tab", "tab", "tab"}, focusSidebar},
		{"shift+tab backwards", 100, []string{"shift+tab"}, focusOutput},
		{"narrow skips the sidebar", 80, []string{"tab", "tab", "tab"}, focusInput},
		{"ctrl+b hides and moves focus", 100, []string{"ctrl+b"}, focusInput},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newApp(t, tt.w, 30)
			press(a, tt.keys...)
			if a.focus != tt.want {
				t.Errorf("focus = %d, want %d", a.focus, tt.want)
			}
		})
	}
}

func TestSidebarCollapse(t *testing.T) {
	a := newApp(t, 100, 30)
	if !a.showSidebar {
		t.Fatal("sidebar hidden at 100 columns")
	}
	send(a, tea.WindowSizeMsg{Width: 89, Height: 30})
	if a.showSidebar {
		t.Error("sidebar visible at 89 columns")
	}
	press(a, "ctrl+b")
	if !a.showSidebar {
		t.Error("ctrl+b does not show the sidebar")
	}
	send(a, tea.WindowSizeMsg{Width: 85, Height: 30})
	if !a.showSidebar {
		t.Error("a resize that did not cross the threshold overrode the user's choice")
	}
}

func TestQuitKeys(t *testing.T) {
	tests := []struct {
		name     string
		keys     []string
		msg      tea.KeyPressMsg
		wantQuit bool
	}{
		{"q in the sidebar quits", nil, tea.KeyPressMsg{Code: 'q', Text: "q"}, true},
		{"q on a select option quits", []string{"tab"}, tea.KeyPressMsg{Code: 'q', Text: "q"}, true},
		{"q in the input types", []string{"tab", "tab"}, tea.KeyPressMsg{Code: 'q', Text: "q"}, false},
		{"ctrl+c in the input quits", []string{"tab", "tab"}, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newApp(t, 100, 30)
			press(a, tt.keys...)
			_, cmd := a.Update(tt.msg)
			quit := cmd != nil && isQuit(cmd())
			if quit != tt.wantQuit {
				t.Errorf("quit = %v, want %v", quit, tt.wantQuit)
			}
		})
	}
}

func TestRunAndClearKeys(t *testing.T) {
	a := newApp(t, 100, 30)
	press(a, "tab", "tab")
	send(a, tea.PasteMsg{Content: strings.Repeat("a", 40000)})
	ws := a.workspace()
	if !ws.Large() {
		t.Fatal("did not enter large mode")
	}
	send(a, tea.KeyPressMsg{Code: tea.KeyF5})
	if ws.Err() != nil {
		t.Fatalf("F5: %v", ws.Err())
	}
	send(a, tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl})
	if ws.Large() || ws.Output() != "" {
		t.Errorf("ctrl+l did not clear the input (Large=%v, output=%d bytes)", ws.Large(), len(ws.Output()))
	}
}

func keyCtrl(r rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: r, Mod: tea.ModCtrl} }

func isQuit(msg tea.Msg) bool {
	_, ok := msg.(tea.QuitMsg)
	return ok
}

func TestOptionsRerun(t *testing.T) {
	tests := []struct {
		name    string
		keys    []string
		want    string
		wantErr bool
	}{
		{"encode by default", []string{"tab", "tab", "hola"}, "aG9sYQ==", false},
		{"switching to decode reruns", []string{"tab", "tab", "aG9sYQ==", "shift+tab", "right"}, "hola", false},
		{"without padding", []string{"tab", "down", "down", "space", "tab", "hola"}, "aG9sYQ", false},
		{"error keeps the previous output", []string{"tab", "right", "tab", "aG9sYQ==", "*"}, "hola", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newApp(t, 100, 30)
			press(a, tt.keys...)
			ws := a.workspace()
			if ws.Output() != tt.want {
				t.Errorf("output = %q, want %q", ws.Output(), tt.want)
			}
			if (ws.Err() != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr %v", ws.Err(), tt.wantErr)
			}
		})
	}
}

// TestProgram walks the real flow with teatest: start, type and quit. The
// v2 renderer only rewrites the cells that change, so the final model is
// checked instead of searching the raw output.
func TestProgram(t *testing.T) {
	reg, err := registry.New(encoders.Base64{})
	if err != nil {
		t.Fatal(err)
	}
	// The filter signals when the result for "hola" arrives, because runs
	// are async and the test cannot quit before that.
	done := make(chan struct{})
	var once sync.Once
	filter := func(_ tea.Model, msg tea.Msg) tea.Msg {
		if r, ok := msg.(workspace.ResultMsg); ok && r.Result.Text == "aG9sYQ==" {
			once.Do(func() { close(done) })
		}
		return msg
	}
	tm := teatest.NewTestModel(t, New(reg, DefaultOptions()),
		teatest.WithInitialTermSize(100, 30),
		teatest.WithProgramOptions(tea.WithFilter(filter)))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return strings.Contains(string(b), "Encoders")
	}, teatest.WithDuration(3*time.Second))
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	tm.Type("hola")
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("the async run result never arrived")
	}
	tm.Send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	final, ok := tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second)).(*App)
	if !ok {
		t.Fatal("the final model is not *App")
	}
	if got := final.workspace().Output(); got != "aG9sYQ==" {
		t.Errorf("output = %q, want %q", got, "aG9sYQ==")
	}
}

func TestHighlightedOutput(t *testing.T) {
	tests := []struct {
		name      string
		paste     string
		wantColor bool
		wantNote  bool
	}{
		{"small json is coloured", `{"a":[1,true,null]}`, true, false},
		{"large json without colour", "[" + strings.Repeat(`"xxxxxxxxxxxxxxxxxxxx",`, 3500) + "1]", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newAppWith(t, 100, 30, formatters.JSONFormat{})
			press(a, "tab", "tab")
			send(a, tea.PasteMsg{Content: tt.paste})
			// Only the output panel lines: between its header and the statusbar.
			lines := strings.Split(a.View().Content, "\n")
			head := slices.IndexFunc(lines, func(l string) bool { return strings.Contains(l, "Output ") })
			out := strings.Join(lines[head+1:len(lines)-1], "\n")
			if got := strings.Contains(out, "\x1b[3"); got != tt.wantColor { // foreground colours 30-37
				t.Errorf("coloured output = %v, want %v", got, tt.wantColor)
			}
			if got := a.workspace().Status().Note != ""; got != tt.wantNote {
				t.Errorf("no-colour notice = %v, want %v", got, tt.wantNote)
			}
		})
	}
}

func TestRegexMatchesView(t *testing.T) {
	a := newAppWith(t, 100, 30, testers.Regex{})
	press(a, "tab", `\d+`, "tab", "a1 b22\nc333")
	content := a.View().Content
	if !strings.Contains(content, "\x1b[7m") {
		t.Error("matches are not highlighted")
	}
	if got := a.workspace().Status().Meta[0].Value; got != "3" {
		t.Errorf("Matches = %s, want 3", got)
	}
	testutil.Golden(t, "tui", "regex-matches-100x30", plainView(a))
}

func TestDiffView(t *testing.T) {
	a := newAppWith(t, 130, 30, text.Diff{})
	press(a, "tab", "tab")
	send(a, tea.PasteMsg{Content: "one\ntwo\nthree\n"})
	press(a, "tab")
	send(a, tea.PasteMsg{Content: "one\nTWO\nthree\nfour\n"})
	if got := a.workspace().Status().Meta[0].Value; got != "+2 −1 lines" {
		t.Errorf("Changes = %q", got)
	}
	if !strings.Contains(a.View().Content, "\x1b[") {
		t.Error("the diff is not coloured")
	}
	testutil.Golden(t, "tui", "diff-130x30", plainView(a))
}

func TestInputErrorExcerpt(t *testing.T) {
	a := newAppWith(t, 100, 30, formatters.JSONFormat{})
	press(a, "tab", "tab")
	send(a, tea.PasteMsg{Content: "{\n  \"a\": [1, 2]\n}"})
	if a.workspace().Err() != nil {
		t.Fatal(a.workspace().Err())
	}
	send(a, tea.PasteMsg{Content: ","})
	if a.workspace().Err() == nil {
		t.Fatal("expected an InputError")
	}
	content := a.View().Content
	if !strings.Contains(content, "\x1b[4") && !strings.Contains(content, ";4m") && !strings.Contains(content, ";4;") {
		t.Error("the culprit character is not underlined")
	}
	raw := ansi.Strip(content)
	if n := strings.Count(raw, "\n") + 1; n != 30 {
		t.Errorf("the view has %d lines, want 30", n)
	}
	testutil.Golden(t, "tui", "json-error-100x30", plainView(a))
}

func TestColorSwatch(t *testing.T) {
	a := newAppWith(t, 100, 30, converters.Color{})
	press(a, "tab", "#663399")
	if content := a.View().Content; !strings.Contains(content, "48;2;102;51;153") {
		t.Error("the output has no swatch")
	}
	if !strings.Contains(plainView(a), "Name:     rebeccapurple") {
		t.Error("the output has no conversions")
	}
}

func TestConfigOptions(t *testing.T) {
	reg, err := registry.New(encoders.Base64{})
	if err != nil {
		t.Fatal(err)
	}
	a := New(reg, Options{
		ToolDefaults: map[string]tool.Values{"base64": {"mode": "decode"}},
		Limits:       workspace.LimitsFor(4 << 10),
		Warnings:     []string{"config.toml: unknown key \"colour\"", "second"},
	})
	send(a, tea.WindowSizeMsg{Width: 100, Height: 30})
	testutil.RunCmds(a.Init(), func(msg tea.Msg) tea.Cmd { _, c := a.Update(msg); return c })
	view := plainView(a)
	if !strings.Contains(view, "Mode: ◀ decode ▶") {
		t.Errorf("the config default was not applied:\n%s", view)
	}
	if a.flash != `config: config.toml: unknown key "colour" (+1 more)` {
		t.Errorf("flash = %q", a.flash)
	}
	// live_max_bytes = 4 KB: a 5 KB paste goes to large mode.
	press(a, "tab", "tab")
	send(a, tea.PasteMsg{Content: strings.Repeat("aG9s", 1300)})
	if !a.workspace().Large() {
		t.Error("the limits from the config were not applied")
	}
}

func TestRestoreSession(t *testing.T) {
	reg, err := registry.New(formatters.JSONFormat{}, encoders.Base64{})
	if err != nil {
		t.Fatal(err)
	}
	a := New(reg, Options{
		ToolDefaults: map[string]tool.Values{"base64": {"variant": "url", "mode": "encode"}},
		Restore: history.Session{
			LastTool: "base64",
			Options:  map[string]tool.Values{"base64": {"mode": "decode"}},
			Inputs:   map[string]map[string]string{"base64": {"input": "aG9sYQ=="}},
		},
	})
	send(a, tea.WindowSizeMsg{Width: 100, Height: 30})
	testutil.RunCmds(a.Init(), func(msg tea.Msg) tea.Cmd { _, c := a.Update(msg); return c })
	if a.active != "base64" {
		t.Fatalf("active = %q, want the last tool", a.active)
	}
	ws := a.workspace()
	if got := ws.Values(); got["mode"] != "decode" || got["variant"] != "url" {
		t.Errorf("options = %v: the session must win over the config, which fills the rest", got)
	}
	if ws.Output() != "hola" {
		t.Errorf("output = %q: the restored input did not run", ws.Output())
	}
	s := a.Session()
	if s.LastTool != "base64" || s.Inputs["base64"]["input"] != "aG9sYQ==" || s.Options["base64"]["mode"] != "decode" {
		t.Errorf("Session = %+v", s)
	}
	if _, ok := s.Options["json-format"]; ok {
		t.Error("Session includes a tool that was never opened")
	}
}

func TestThemes(t *testing.T) {
	newThemed := func(theme string) *App {
		reg, err := registry.New(formatters.JSONFormat{})
		if err != nil {
			t.Fatal(err)
		}
		a := New(reg, Options{Theme: theme})
		send(a, tea.WindowSizeMsg{Width: 100, Height: 30})
		testutil.RunCmds(a.Init(), func(msg tea.Msg) tea.Cmd { _, c := a.Update(msg); return c })
		press(a, "tab", "tab", `{"a":1}`)
		return a
	}
	// The statusbar shows the run time, which changes on each run.
	noStatus := func(a *App) string {
		c := a.View().Content
		return c[:strings.LastIndex(c, "\n")]
	}
	white := tea.BackgroundColorMsg{Color: color.White}
	black := tea.BackgroundColorMsg{Color: color.Black}

	if a := newThemed("light"); a.theme.Dark {
		t.Error(`theme "light" started dark`)
	}
	dark := newThemed("dark")
	before := noStatus(dark)
	send(dark, white)
	if !dark.theme.Dark || noStatus(dark) != before {
		t.Error(`theme "dark" followed the terminal background`)
	}

	auto := newThemed("auto")
	if !auto.theme.Dark {
		t.Fatal(`theme "auto" must start dark until the terminal answers`)
	}
	send(auto, white)
	if auto.theme.Dark {
		t.Fatal("auto did not switch to light on a white background")
	}
	light := noStatus(auto)
	if light == before {
		t.Error("the view did not change with the light theme")
	}
	if plainView(auto) != plainView(dark) {
		t.Error("the theme changed the text, not only the colours")
	}
	send(auto, black)
	if !auto.theme.Dark || noStatus(auto) != before {
		t.Error("auto did not switch back to dark")
	}
}

func TestMarkdownRewraps(t *testing.T) {
	a := newAppWith(t, 140, 30, text.Markdown{})
	press(a, "tab")
	send(a, tea.PasteMsg{Content: "# Title\n\n" + strings.Repeat("word ", 60)})
	output := func() string { // the view below the output header
		_, out, _ := strings.Cut(plainView(a), "── Output")
		return out
	}
	wide := output()
	if !strings.Contains(wide, "Title") || strings.Contains(wide, "# Title") {
		t.Fatalf("the output is not rendered Markdown:\n%s", wide)
	}
	send(a, tea.WindowSizeMsg{Width: 100, Height: 30})
	narrow := output()
	longest := func(s string) int {
		n := 0
		for _, l := range strings.Split(s, "\n") {
			if i := strings.Index(l, "word"); i >= 0 {
				n = max(n, strings.Count(l[i:], "word"))
			}
		}
		return n
	}
	if longest(narrow) >= longest(wide) {
		t.Errorf("the output did not rewrap: %d words per line at 140 columns, %d at 100", longest(wide), longest(narrow))
	}
}
