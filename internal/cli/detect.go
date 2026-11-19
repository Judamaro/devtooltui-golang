package cli

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Judamaro/devtooltui-golang/internal/detect"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

type candidateJSON struct {
	ID    string  `json:"id"`
	Score float64 `json:"score"`
}

type detectJSON struct {
	Best       *candidateJSON  `json:"best"`
	Ambiguous  bool            `json:"ambiguous"`
	Candidates []candidateJSON `json:"candidates"`
}

func newDetect(d Deps, g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "detect [file|-]",
		Short:   "Detect which tool matches the input",
		Long:    "Analyses a sample of up to 64 KB and picks the tool with the highest\nscore above 0.5; when two are less than 0.1 apart, it lists them all.",
		GroupID: "general",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			spec := tool.Spec{ID: "detect", Inputs: []tool.InputSpec{{Key: "input"}}}
			inputs, _, err := readInputs(spec, args, d, g.maxInput)
			if err != nil {
				return err
			}
			out := detect.Run(d.Registry, inputs["input"])
			w := cmd.OutOrStdout()
			if g.json {
				dj := detectJSON{Ambiguous: out.Ambiguous, Candidates: []candidateJSON{}}
				if out.Best != nil {
					dj.Best = &candidateJSON{out.Best.Tool.Spec().ID, out.Best.Score}
				}
				for _, c := range out.Candidates {
					dj.Candidates = append(dj.Candidates, candidateJSON{c.Tool.Spec().ID, c.Score})
				}
				enc := json.NewEncoder(w)
				enc.SetIndent("", "  ")
				return enc.Encode(dj)
			}
			switch {
			case out.Best != nil:
				fmt.Fprintf(w, "%s (%.2f)\n", out.Best.Tool.Spec().ID, out.Best.Score)
			case out.Ambiguous:
				fmt.Fprintln(cmd.ErrOrStderr(), "devtooltui: several tools match:")
				for _, c := range out.Candidates {
					fmt.Fprintf(w, "%s (%.2f)\n", c.Tool.Spec().ID, c.Score)
				}
			default:
				return &exitError{ExitInput, errors.New("the input format was not recognised")}
			}
			return nil
		},
	}
}
