// Command devtooltui bundles developer utilities in a TUI and a CLI.
package main

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"
	"time"

	"github.com/charmbracelet/x/term"

	"github.com/Judamaro/devtooltui-golang/internal/cli"
	"github.com/Judamaro/devtooltui-golang/internal/clipboard"
	"github.com/Judamaro/devtooltui-golang/internal/config"
	"github.com/Judamaro/devtooltui-golang/internal/history"
	"github.com/Judamaro/devtooltui-golang/internal/plugin"
	"github.com/Judamaro/devtooltui-golang/internal/registry"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools"
	"github.com/Judamaro/devtooltui-golang/internal/tui"
	"github.com/Judamaro/devtooltui-golang/internal/tui/workspace"
)

// Injected with -ldflags at build time (make build, GoReleaser); without
// them, buildVersion reads what Go recorded in the binary.
var (
	version = "dev"
	commit  = "none"
)

func main() {
	bi, _ := debug.ReadBuildInfo()
	version, commit = buildVersion(version, commit, bi)
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	pathList := os.Getenv("PATH")
	pluginsOff := false
	if cfg, _, err := config.Path(cli.ConfigFlag(args), os.Getenv, os.UserConfigDir); err == nil && !config.PluginsEnabled(cfg) {
		pathList, pluginsOff = "", true
	}
	cacheFile, _ := plugin.CachePath(os.UserCacheDir) // "" without a cache dir: no cache
	reg, plugins, err := plugin.Registry(context.Background(), pathList, cacheFile, tools.All())
	if err != nil {
		fmt.Fprintln(os.Stderr, "devtooltui: invalid registry:", err)
		return cli.ExitInternal
	}
	clip := clipboard.New(os.Getenv, os.Stderr)
	return cli.Execute(args, cli.Deps{
		Registry:          reg,
		Stdin:             os.Stdin,
		Stdout:            os.Stdout,
		Stderr:            os.Stderr,
		StdinTTY:          term.IsTerminal(os.Stdin.Fd()),
		StdoutTTY:         term.IsTerminal(os.Stdout.Fd()),
		Getenv:            os.Getenv,
		RenderStyles:      tui.RenderStyles(),
		LightRenderStyles: tui.RenderStylesFor(false),
		TermWidth: func() int {
			if w, _, err := term.GetSize(os.Stdout.Fd()); err == nil && w > 0 {
				return min(w, 120)
			}
			return 80
		},
		RunTUI:        func(r cli.TUIRequest) error { return runTUI(reg, r) },
		Clipboard:     clip,
		Plugins:       plugins,
		PluginsOff:    pluginsOff,
		UserConfigDir: os.UserConfigDir,
		Version:       version,
		Commit:        commit,
	})
}

// runTUI applies the config, restores the history and saves it on exit.
func runTUI(reg *registry.Registry, r cli.TUIRequest) error {
	opts := tui.DefaultOptions()
	opts.Clipboard = clipboard.New(os.Getenv, nil) // in the TUI, OSC 52 goes through tea.SetClipboard
	opts.DetectOnStart = r.Clipboard || r.Config.DetectOnStart
	opts.Debounce = time.Duration(r.Config.DebounceMS) * time.Millisecond
	opts.ToolDefaults = r.Config.Tools
	opts.Limits = workspace.LimitsFor(r.Config.LiveMaxBytes)
	opts.Warnings = r.Warnings
	opts.Theme = r.Config.Theme
	opts.Keys = r.Config.Keys

	hist := r.Config.History
	statePath, err := history.Path(os.UserCacheDir)
	if !hist.Enabled || err != nil {
		_, err := tui.Run(reg, opts)
		return err
	}
	prev := history.Load(statePath, reg)
	opts.Restore.LastTool = prev.LastTool
	opts.Restore.Options = map[string]tool.Values{}
	opts.Restore.Inputs = map[string]map[string]string{}
	for id, ts := range prev.Tools {
		if hist.RememberOptions {
			opts.Restore.Options[id] = ts.Options
		}
		if hist.StoreInputs {
			opts.Restore.Inputs[id] = ts.Inputs
		}
	}
	sess, err := tui.Run(reg, opts)
	if err != nil {
		return err
	}
	if err := history.Save(statePath, history.Merge(prev, sess, hist)); err != nil {
		fmt.Fprintln(os.Stderr, "devtooltui: could not save the history:", err)
	}
	return nil
}
