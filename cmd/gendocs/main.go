// Command gendocs writes the tool pages of the documentation site and the
// man pages from the registry and the CLI, so they cannot drift from the
// code:
//
//	go run ./cmd/gendocs -site docs   # docs/index.md and docs/tools/<id>.md
//	go run ./cmd/gendocs -man man     # man/devtooltui*.1
//
// Examples come from the golden files in testdata/<id>/, which the tests
// keep true.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra/doc"

	"github.com/Judamaro/devtooltui-golang/internal/cli"
	"github.com/Judamaro/devtooltui-golang/internal/registry"
	"github.com/Judamaro/devtooltui-golang/internal/tools"
)

func main() {
	site := flag.String("site", "", "directory for index.md and tools/<id>.md")
	man := flag.String("man", "", "directory for the man pages")
	testdata := flag.String("testdata", "testdata", "directory with the golden files")
	flag.Parse()
	if *site == "" && *man == "" {
		fmt.Fprintln(os.Stderr, "gendocs: pass -site DIR and/or -man DIR")
		os.Exit(2)
	}
	if err := run(*site, *man, *testdata); err != nil {
		fmt.Fprintln(os.Stderr, "gendocs:", err)
		os.Exit(1)
	}
}

func run(site, man, testdata string) error {
	reg, err := registry.New(tools.All()...)
	if err != nil {
		return err
	}
	if site != "" {
		if err := writeSite(site, testdata, reg); err != nil {
			return err
		}
	}
	if man != "" {
		if err := os.MkdirAll(man, 0o755); err != nil { //nolint:gosec // man pages are public
			return err
		}
		root := cli.NewRoot(cli.Deps{Registry: reg, Stdout: io.Discard, Stderr: io.Discard, Getenv: os.Getenv})
		root.DisableAutoGenTag = true // no date: the pages are reproducible
		header := &doc.GenManHeader{Title: "DEVTOOLTUI", Section: "1", Source: "devtooltui", Manual: "devtooltui manual"}
		if err := doc.GenManTree(root, header, filepath.Clean(man)); err != nil {
			return err
		}
	}
	return nil
}
