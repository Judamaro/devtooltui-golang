// Package cli builds the command-line interface from the registry: every
// Spec becomes a cobra subcommand with its flags.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/Judamaro/devtooltui-golang/internal/clipboard"
	"github.com/Judamaro/devtooltui-golang/internal/registry"
	"github.com/Judamaro/devtooltui-golang/internal/render"
)

// Exit codes.
const (
	ExitOK       = 0
	ExitInput    = 1 // invalid input
	ExitUsage    = 2 // usage error
	ExitInternal = 3 // internal error
)

// DefaultMaxInput is the --max-input limit (64 MB).
const DefaultMaxInput = 64 << 20

// Deps is everything the CLI takes from the environment; tests replace it.
type Deps struct {
	Registry        *registry.Registry
	Stdin           io.Reader
	Stdout, Stderr  io.Writer
	StdinTTY        bool
	StdoutTTY       bool
	Getenv          func(string) string
	RenderStyles    render.Styles
	RunTUI          func(clipboard bool) error // nil: no TUI available
	Clipboard       clipboard.Clipboard        // for --copy
	Version, Commit string
}

// exitError carries the exit code along with the error.
type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

func usageErr(format string, a ...any) error {
	return &exitError{ExitUsage, fmt.Errorf(format, a...)}
}

// Execute runs the CLI with args and returns the exit code.
func Execute(args []string, d Deps) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	root := newRoot(d)
	root.SetArgs(args)
	root.SetIn(d.Stdin)
	root.SetOut(d.Stdout)
	root.SetErr(d.Stderr)
	err := root.ExecuteContext(ctx)
	if err == nil {
		return ExitOK
	}
	var ee *exitError
	if errors.As(err, &ee) {
		if ee.err != nil && ee.err.Error() != "" {
			fmt.Fprintln(d.Stderr, "devtooltui:", ee.err)
		}
		return ee.code
	}
	// Cobra errors (unknown command or flag, arguments): usage.
	fmt.Fprintln(d.Stderr, "devtooltui:", err)
	return ExitUsage
}

type globalFlags struct {
	json     bool
	copy     bool
	noColor  bool
	maxInput int64
}

func newRoot(d Deps) *cobra.Command {
	var g globalFlags
	var clip bool
	root := &cobra.Command{
		Use:   "devtooltui",
		Short: "Developer utilities in the terminal: a TUI and a composable CLI",
		Long: "devtooltui bundles formatters, encoders, converters and testers.\n" +
			"Without arguments and in a terminal it opens the TUI; with a subcommand it\n" +
			"works as a filter: it reads a file or stdin and writes to stdout.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !d.StdinTTY || !d.StdoutTTY || d.RunTUI == nil {
				return usageErr("the TUI needs a terminal; use a subcommand (devtooltui list)")
			}
			if err := d.RunTUI(clip); err != nil {
				return &exitError{ExitInternal, err}
			}
			return nil
		},
	}
	root.Flags().BoolVar(&clip, "clipboard", false, "open the TUI on the tool detected in the clipboard")
	root.PersistentFlags().BoolVar(&g.json, "json", false, "JSON output (the full Result)")
	root.PersistentFlags().BoolVar(&g.noColor, "no-color", false, "no colours (also with NO_COLOR)")
	root.PersistentFlags().BoolVar(&g.copy, "copy", false, "copy the output to the clipboard")
	root.PersistentFlags().Int64Var(&g.maxInput, "max-input", DefaultMaxInput, "maximum input size in bytes")
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return &exitError{ExitUsage, err} })
	setupHelp(root)
	root.CompletionOptions.HiddenDefaultCmd = false

	root.AddGroup(&cobra.Group{ID: "tools", Title: "Tools:"}, &cobra.Group{ID: "general", Title: "General:"})
	root.SetHelpCommandGroupID("general")
	root.SetCompletionCommandGroupID("general")
	root.AddCommand(newVersion(d), newList(d, &g), newDetect(d, &g))
	for _, t := range d.Registry.All() {
		root.AddCommand(newToolCmd(t, d, &g))
	}
	return root
}

func newVersion(d Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "version",
		Short:   "Show the version, commit, Go version and platform",
		GroupID: "general",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "devtooltui %s (%s) %s %s/%s\n", d.Version, d.Commit, runtime.Version(), runtime.GOOS, runtime.GOARCH)
			return nil
		},
	}
}
