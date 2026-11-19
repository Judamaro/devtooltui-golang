package cli

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

type optionJSON struct {
	Key     string   `json:"key"`
	Flag    string   `json:"flag"`
	Kind    string   `json:"kind"`
	Choices []string `json:"choices,omitempty"`
	Min     *int     `json:"min,omitempty"`
	Max     *int     `json:"max,omitempty"`
	Default any      `json:"default"`
	Help    string   `json:"help,omitempty"`
}

type toolJSON struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Category    string       `json:"category"`
	Description string       `json:"description"`
	Inputs      []string     `json:"inputs"`
	Options     []optionJSON `json:"options"`
}

var kindNames = []string{"toggle", "select", "number", "string"}

func newList(d Deps, g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List the available tools",
		GroupID: "general",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			if g.json {
				var all []toolJSON
				for _, t := range d.Registry.All() {
					s := t.Spec()
					tj := toolJSON{ID: s.ID, Name: s.Name, Category: string(s.Category), Description: s.Description,
						Inputs: []string{}, Options: []optionJSON{}}
					for _, in := range s.Inputs {
						tj.Inputs = append(tj.Inputs, in.Key)
					}
					for _, o := range s.Options {
						oj := optionJSON{Key: o.Key, Flag: "--" + flagName(o.Key), Kind: kindNames[o.Kind],
							Choices: o.Choices, Default: o.Default, Help: o.Help}
						if o.Kind == tool.Number {
							oj.Min, oj.Max = &o.Min, &o.Max
						}
						tj.Options = append(tj.Options, oj)
					}
					all = append(all, tj)
				}
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				enc.SetEscapeHTML(false)
				return enc.Encode(all)
			}
			tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			for _, grp := range d.Registry.ByCategory() {
				for _, t := range grp.Tools {
					s := t.Spec()
					fmt.Fprintf(tw, "%s\t%s\t%s\n", s.ID, grp.Category, s.Description)
				}
			}
			return tw.Flush()
		},
	}
}
