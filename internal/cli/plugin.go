package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/Judamaro/devtooltui-golang/internal/plugin"
)

type pluginJSON struct {
	Name     string   `json:"name"`
	Path     string   `json:"path"`
	Protocol int      `json:"protocol"`
	Tools    []string `json:"tools"`
	Problems []string `json:"problems"`
}

func newPlugin(d Deps, g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "plugin",
		Short:   "External tools (devtooltui-<name> executables in the PATH)",
		GroupID: "general",
		Args:    cobra.NoArgs,
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List the plugins found in the PATH, their tools and their problems",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			if g.json {
				all := []pluginJSON{}
				for _, p := range d.Plugins {
					all = append(all, pluginJSON{Name: p.Name, Path: p.Path, Protocol: p.Protocol,
						Tools: nonNil(p.Tools), Problems: nonNil(p.Problems)})
				}
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				enc.SetEscapeHTML(false)
				return enc.Encode(all)
			}
			if d.PluginsOff {
				fmt.Fprintln(out, "Plugins are turned off: [plugins] enabled = false in the config.")
				return nil
			}
			if len(d.Plugins) == 0 {
				fmt.Fprintf(out, "No plugins: put %s<name> executables in the PATH (see docs/plugins.md).\n", plugin.Prefix)
				return nil
			}
			tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "PLUGIN\tTOOLS\tPATH")
			for _, p := range d.Plugins {
				tools := strings.Join(p.Tools, ", ")
				if tools == "" {
					tools = "-"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\n", p.Name, tools, p.Path)
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			for _, p := range d.Plugins {
				for _, problem := range p.Problems {
					fmt.Fprintf(out, "%s: %s\n", p.Name, problem)
				}
			}
			return nil
		},
	})
	return cmd
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
