// Command devtooltui reúne utilidades de desarrollo en una TUI y una CLI.
package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/x/term"

	"github.com/Judamaro/devtooltui-golang/internal/cli"
	"github.com/Judamaro/devtooltui-golang/internal/registry"
	"github.com/Judamaro/devtooltui-golang/internal/tools"
	"github.com/Judamaro/devtooltui-golang/internal/tui"
)

// Inyectadas con -ldflags en el build.
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
		fmt.Fprintln(os.Stderr, "devtooltui: registro inválido:", err)
		return cli.ExitInternal
	}
	return cli.Execute(args, cli.Deps{
		Registry:     reg,
		Stdin:        os.Stdin,
		Stdout:       os.Stdout,
		Stderr:       os.Stderr,
		StdinTTY:     term.IsTerminal(os.Stdin.Fd()),
		StdoutTTY:    term.IsTerminal(os.Stdout.Fd()),
		Getenv:       os.Getenv,
		RenderStyles: tui.RenderStyles(),
		RunTUI:       func(bool) error { return tui.Run(reg) },
		Version:      version,
		Commit:       commit,
	})
}
