// Package cli genera la interfaz de línea de comandos desde el registro:
// cada Spec produce un subcomando de cobra con sus flags.
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

	"github.com/Judamaro/devtooltui-golang/internal/registry"
	"github.com/Judamaro/devtooltui-golang/internal/render"
)

// Códigos de salida.
const (
	ExitOK       = 0
	ExitInput    = 1 // entrada inválida
	ExitUsage    = 2 // uso incorrecto
	ExitInternal = 3 // error interno
)

// DefaultMaxInput es el límite de --max-input (64 MB).
const DefaultMaxInput = 64 << 20

// Deps es todo lo que la CLI toma del entorno; los tests lo reemplazan.
type Deps struct {
	Registry        *registry.Registry
	Stdin           io.Reader
	Stdout, Stderr  io.Writer
	StdinTTY        bool
	StdoutTTY       bool
	Getenv          func(string) string
	RenderStyles    render.Styles
	RunTUI          func(clipboard bool) error // nil: no hay TUI disponible
	Version, Commit string
}

// exitError lleva el código de salida junto con el error.
type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

func usageErr(format string, a ...any) error {
	return &exitError{ExitUsage, fmt.Errorf(format, a...)}
}

// Execute corre la CLI con args y devuelve el código de salida.
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
	// Errores de cobra (comando o flag desconocidos, argumentos): uso.
	fmt.Fprintln(d.Stderr, "devtooltui:", err)
	return ExitUsage
}

type globalFlags struct {
	json     bool
	noColor  bool
	maxInput int64
}

func newRoot(d Deps) *cobra.Command {
	var g globalFlags
	var clip bool
	root := &cobra.Command{
		Use:   "devtooltui",
		Short: "Utilidades de desarrollo en la terminal: TUI y CLI componible",
		Long: "devtooltui reúne formateadores, codificadores, conversores y testers.\n" +
			"Sin argumentos y en una terminal abre la TUI; con un subcomando funciona\n" +
			"como filtro: lee un archivo o stdin y escribe en stdout.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !d.StdinTTY || !d.StdoutTTY || d.RunTUI == nil {
				return usageErr("la TUI necesita una terminal; usa un subcomando (devtooltui list)")
			}
			if err := d.RunTUI(clip); err != nil {
				return &exitError{ExitInternal, err}
			}
			return nil
		},
	}
	root.Flags().BoolVar(&clip, "clipboard", false, "abrir la TUI con la herramienta detectada en el portapapeles")
	root.PersistentFlags().BoolVar(&g.json, "json", false, "salida en JSON (el Result completo)")
	root.PersistentFlags().BoolVar(&g.noColor, "no-color", false, "sin colores (también con NO_COLOR)")
	root.PersistentFlags().Int64Var(&g.maxInput, "max-input", DefaultMaxInput, "tamaño máximo de entrada en bytes")
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return &exitError{ExitUsage, err} })
	spanishHelp(root)
	root.CompletionOptions.HiddenDefaultCmd = false

	root.AddGroup(&cobra.Group{ID: "tools", Title: "Herramientas:"}, &cobra.Group{ID: "general", Title: "Generales:"})
	root.SetHelpCommandGroupID("general")
	root.SetCompletionCommandGroupID("general")
	root.AddCommand(newVersion(d), newList(d, &g))
	for _, t := range d.Registry.All() {
		root.AddCommand(newToolCmd(t, d, &g))
	}
	return root
}

func newVersion(d Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "version",
		Short:   "Muestra la versión, el commit, Go y la plataforma",
		GroupID: "general",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "devtooltui %s (%s) %s %s/%s\n", d.Version, d.Commit, runtime.Version(), runtime.GOOS, runtime.GOARCH)
			return nil
		},
	}
}
