// Command devtooltui reúne utilidades de desarrollo en una TUI y una CLI.
package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/charmbracelet/x/term"

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
	if len(args) == 1 && args[0] == "version" {
		fmt.Printf("devtooltui %s (%s) %s %s/%s\n", version, commit, runtime.Version(), runtime.GOOS, runtime.GOARCH)
		return 0
	}
	reg, err := registry.New(tools.All()...)
	if err != nil {
		fmt.Fprintln(os.Stderr, "devtooltui: registro inválido:", err)
		return 3
	}
	if len(args) == 0 && term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stdout.Fd()) {
		if err := tui.Run(reg); err != nil {
			fmt.Fprintln(os.Stderr, "devtooltui:", err)
			return 3
		}
		return 0
	}
	fmt.Fprintln(os.Stderr, "devtooltui: la CLI llega en la fase 1")
	return 2
}
