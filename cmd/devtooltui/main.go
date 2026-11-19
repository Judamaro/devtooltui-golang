// Command devtooltui bundles developer utilities in a TUI and a CLI.
package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/x/term"

	"github.com/Judamaro/devtooltui-golang/internal/cli"
	"github.com/Judamaro/devtooltui-golang/internal/clipboard"
	"github.com/Judamaro/devtooltui-golang/internal/registry"
	"github.com/Judamaro/devtooltui-golang/internal/tools"
	"github.com/Judamaro/devtooltui-golang/internal/tui"
)

// Injected with -ldflags at build time.
var (
	version = "dev"
	commit  = "none"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	reg, err := registry.New(tools.All()...)
	if err != nil {
		fmt.Fprintln(os.Stderr, "devtooltui: invalid registry:", err)
		return cli.ExitInternal
	}
	clip := clipboard.New(os.Getenv, os.Stderr)
	return cli.Execute(args, cli.Deps{
		Registry:     reg,
		Stdin:        os.Stdin,
		Stdout:       os.Stdout,
		Stderr:       os.Stderr,
		StdinTTY:     term.IsTerminal(os.Stdin.Fd()),
		StdoutTTY:    term.IsTerminal(os.Stdout.Fd()),
		Getenv:       os.Getenv,
		RenderStyles: tui.RenderStyles(),
		RunTUI: func(detect bool) error {
			opts := tui.DefaultOptions()
			opts.Clipboard = clipboard.New(os.Getenv, nil) // in the TUI, OSC 52 goes through tea.SetClipboard
			opts.DetectOnStart = detect
			return tui.Run(reg, opts)
		},
		Clipboard: clip,
		Version:   version,
		Commit:    commit,
	})
}
