package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/Judamaro/devtooltui-golang/internal/clipboard"
	"github.com/Judamaro/devtooltui-golang/internal/render"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// flagName turns an option key into a flag: sort_keys → sort-keys.
func flagName(key string) string { return strings.ReplaceAll(key, "_", "-") }

func newToolCmd(t tool.Tool, d Deps, g *globalFlags) *cobra.Command {
	s := t.Spec()
	use := s.ID
	if hasMode(s) {
		use += " [" + strings.Join(modeChoices(s), "|") + "]"
	}
	switch len(s.Inputs) {
	case 1:
		use += " [file|-]"
	case 2:
		use += " [file-A|-] [file-B|-]"
	}
	cmd := &cobra.Command{
		Use:     use,
		Short:   s.Description,
		GroupID: "tools",
		Long:    longHelp(s),
	}
	vals := map[string]any{}
	for _, o := range s.Options {
		name := flagName(o.Key)
		switch o.Kind {
		case tool.Toggle:
			def, _ := o.Default.(bool)
			vals[o.Key] = cmd.Flags().Bool(name, def, o.Label+helpSuffix(o))
		case tool.Number:
			def, _ := o.Default.(int)
			vals[o.Key] = cmd.Flags().Int(name, def, fmt.Sprintf("%s (%d–%d)%s", o.Label, o.Min, o.Max, helpSuffix(o)))
		case tool.Select:
			def, _ := o.Default.(string)
			vals[o.Key] = cmd.Flags().String(name, def, fmt.Sprintf("%s: %s%s", o.Label, strings.Join(o.Choices, ", "), helpSuffix(o)))
			choices := o.Choices
			_ = cmd.RegisterFlagCompletionFunc(name, func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
				return choices, cobra.ShellCompDirectiveNoFileComp
			})
		case tool.String:
			def, _ := o.Default.(string)
			vals[o.Key] = cmd.Flags().String(name, def, o.Label+helpSuffix(o))
		}
	}
	cmd.ValidArgsFunction = func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 && hasMode(s) {
			return modeChoices(s), cobra.ShellCompDirectiveDefault
		}
		return nil, cobra.ShellCompDirectiveDefault
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		opts, args, err := collectOptions(s, cmd.Flags(), vals, args)
		if err != nil {
			return err
		}
		inputs, sources, err := readInputs(s, args, d, g.maxInput)
		if err != nil {
			return err
		}
		res, err := t.Run(cmd.Context(), tool.Request{Inputs: inputs, Opts: opts})
		if err != nil {
			return runError(err, sources)
		}
		if err := writeResult(cmd.OutOrStdout(), res, s.Output, inputs, d, g); err != nil {
			return err
		}
		if g.copy {
			copyOutput(cmd.ErrOrStderr(), res.Text, d)
		}
		return nil
	}
	return cmd
}

func hasMode(s tool.Spec) bool { return len(modeChoices(s)) > 0 }

func modeChoices(s tool.Spec) []string {
	for _, o := range s.Options {
		if o.Key == "mode" && o.Kind == tool.Select {
			return o.Choices
		}
	}
	return nil
}

func helpSuffix(o tool.Option) string {
	if o.Help == "" {
		return ""
	}
	return " — " + o.Help
}

func longHelp(s tool.Spec) string {
	var b strings.Builder
	b.WriteString(s.Name + ": " + s.Description + ".")
	switch len(s.Inputs) {
	case 0:
	case 1:
		b.WriteString("\n\nReads " + strings.ToLower(s.Inputs[0].Label) + " from the given file, from \"-\" or from stdin.")
	default:
		b.WriteString("\n\nInputs, in order: ")
		var labels []string
		for _, in := range s.Inputs {
			labels = append(labels, in.Label)
		}
		b.WriteString(strings.Join(labels, "; ") + ". A missing one is read from stdin when stdin is not a terminal.")
	}
	return b.String()
}

// collectOptions builds tool.Values from the flags; "mode" is also accepted
// as the first argument (devtooltui base64 decode).
func collectOptions(s tool.Spec, fs *pflag.FlagSet, vals map[string]any, args []string) (tool.Values, []string, error) {
	opts := tool.Values{}
	for _, o := range s.Options {
		switch v := vals[o.Key].(type) {
		case *bool:
			opts[o.Key] = *v
		case *int:
			if *v < o.Min || *v > o.Max {
				return nil, nil, usageErr("--%s must be between %d and %d", flagName(o.Key), o.Min, o.Max)
			}
			opts[o.Key] = *v
		case *string:
			if o.Kind == tool.Select && !slices.Contains(o.Choices, *v) {
				return nil, nil, usageErr("--%s must be one of: %s", flagName(o.Key), strings.Join(o.Choices, ", "))
			}
			opts[o.Key] = *v
		}
	}
	if choices := modeChoices(s); len(args) > 0 && slices.Contains(choices, args[0]) {
		if fs.Changed("mode") && opts.String("mode") != args[0] {
			return nil, nil, usageErr("mode %q and --mode %q do not match", args[0], opts.String("mode"))
		}
		opts["mode"] = args[0]
		args = args[1:]
	}
	return opts, args, nil
}

// readInputs assigns the arguments to the inputs in order ("-" = stdin).
// The first input left without an argument reads stdin when it is not a terminal.
func readInputs(s tool.Spec, args []string, d Deps, maxInput int64) (map[string]string, map[string]string, error) {
	if len(args) > len(s.Inputs) {
		return nil, nil, usageErr("%s accepts at most %d file(s), got %d", s.ID, len(s.Inputs), len(args))
	}
	inputs := map[string]string{}
	sources := map[string]string{}
	stdinUsed := false
	read := func(key, name string, r io.Reader) error {
		b, err := io.ReadAll(io.LimitReader(r, maxInput+1))
		if err != nil {
			return &exitError{ExitInput, fmt.Errorf("%s: %w", name, err)}
		}
		if int64(len(b)) > maxInput {
			return &exitError{ExitInput, fmt.Errorf("%s: the input exceeds --max-input (%d bytes)", name, maxInput)}
		}
		inputs[key], sources[key] = string(b), name
		return nil
	}
	for i, in := range s.Inputs {
		switch {
		case i < len(args) && args[i] == "-":
			if stdinUsed {
				return nil, nil, usageErr("stdin (\"-\") can only be used once")
			}
			stdinUsed = true
			if err := read(in.Key, "<stdin>", d.Stdin); err != nil {
				return nil, nil, err
			}
		case i < len(args):
			f, err := os.Open(args[i])
			if err != nil {
				return nil, nil, &exitError{ExitInput, err}
			}
			err = read(in.Key, args[i], f)
			_ = f.Close()
			if err != nil {
				return nil, nil, err
			}
		case !stdinUsed && !d.StdinTTY:
			stdinUsed = true
			if err := read(in.Key, "<stdin>", d.Stdin); err != nil {
				return nil, nil, err
			}
		default:
			inputs[in.Key] = ""
		}
	}
	return inputs, sources, nil
}

func runError(err error, sources map[string]string) error {
	ie, ok := tool.AsInputError(err)
	if !ok {
		if errors.Is(err, context.Canceled) {
			return &exitError{ExitInternal, errors.New("cancelled")}
		}
		return &exitError{ExitInternal, fmt.Errorf("internal error: %w", err)}
	}
	src := sources[ie.Input]
	if src == "" {
		src = "--" + flagName(ie.Input) // error in an option (e.g. --pattern)
	}
	return &exitError{ExitInput, fmt.Errorf("%s:%s", src, ie.Error())}
}

type resultJSON struct {
	Text  string     `json:"text"`
	Lang  string     `json:"lang,omitempty"`
	Meta  []kvJSON   `json:"meta"`
	Diff  []diffJSON `json:"diff,omitempty"`
	Spans []spanJSON `json:"spans,omitempty"`
}

type kvJSON struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type diffJSON struct {
	Op   string `json:"op"`
	Text string `json:"text"`
}

type spanJSON struct {
	Input string `json:"input"`
	Start int    `json:"start"`
	End   int    `json:"end"`
	Group int    `json:"group"`
}

var diffOps = []string{"equal", "insert", "delete"}

func writeResult(w io.Writer, res tool.Result, kind tool.OutputKind, inputs map[string]string, d Deps, g *globalFlags) error {
	if g.json {
		rj := resultJSON{Text: res.Text, Lang: res.Lang, Meta: []kvJSON{}}
		for _, kv := range res.Meta {
			rj.Meta = append(rj.Meta, kvJSON(kv))
		}
		for _, op := range res.Diff {
			rj.Diff = append(rj.Diff, diffJSON{Op: diffOps[op.Kind], Text: op.Text})
		}
		for _, sp := range res.Spans {
			rj.Spans = append(rj.Spans, spanJSON(sp))
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		return enc.Encode(rj)
	}
	out := res.Text
	if useColor(d, g) {
		if colored, _ := render.Result(res, kind, inputs, d.RenderStyles); colored != "" {
			out = colored
		}
	}
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	_, err := io.WriteString(w, out)
	return err
}

// copyOutput copies to the clipboard; on failure it only warns on stderr
// (the output was already written).
func copyOutput(stderr io.Writer, text string, d Deps) {
	if d.Clipboard == nil {
		fmt.Fprintln(stderr, "devtooltui: no clipboard available")
		return
	}
	if err := d.Clipboard.Write(text); err != nil {
		fmt.Fprintln(stderr, "devtooltui: could not copy:", err)
		return
	}
	if sys, ok := d.Clipboard.(*clipboard.System); ok && !sys.SSH() && sys.UseOSC52() {
		fmt.Fprintln(stderr, "devtooltui: copied with OSC 52; for the system clipboard,", sys.InstallHint())
	}
}

// useColor: only when stdout is a terminal, without --no-color or NO_COLOR.
func useColor(d Deps, g *globalFlags) bool {
	return d.StdoutTTY && !g.noColor && d.Getenv("NO_COLOR") == ""
}
