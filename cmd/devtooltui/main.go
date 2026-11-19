// Command devtooltui reúne utilidades de desarrollo en una TUI y una CLI.
package main

import (
	"fmt"
	"os"
	"runtime"
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
	fmt.Fprintln(os.Stderr, "devtooltui: la CLI llega en la fase 1")
	return 2
}
