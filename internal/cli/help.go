package cli

import "github.com/spf13/cobra"

// usageTemplate es la plantilla de cobra traducida al español.
const usageTemplate = `Uso:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [comando]{{end}}{{if gt (len .Aliases) 0}}

Alias:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Ejemplos:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Comandos:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Opciones:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Opciones globales:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableSubCommands}}

Usa "{{.CommandPath}} [comando] --help" para ver la ayuda de un comando.{{end}}
`

func spanishHelp(root *cobra.Command) {
	root.SetUsageTemplate(usageTemplate)
	root.PersistentFlags().BoolP("help", "h", false, "muestra esta ayuda")
	root.Example = `  devtooltui                                  # TUI (en una terminal)
  cat data.json | devtooltui json-format --sort-keys
  devtooltui base64 decode <<< 'aG9sYQ=='
  devtooltui jwt --secret "$KEY" < token.txt
  devtooltui diff a.txt b.txt
  devtooltui uuid --version v7 --count 5`
}
