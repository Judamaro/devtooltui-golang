package cli

import "github.com/spf13/cobra"

// setupHelp adds the help flag description and the root examples. Cobra's
// default usage template is already in English.
func setupHelp(root *cobra.Command) {
	root.PersistentFlags().BoolP("help", "h", false, "show this help")
	root.Example = `  devtooltui                                  # TUI (in a terminal)
  cat data.json | devtooltui json-format --sort-keys
  devtooltui base64 decode <<< 'aG9sYQ=='
  devtooltui jwt --secret "$KEY" < token.txt
  devtooltui diff a.txt b.txt
  devtooltui uuid --version v7 --count 5`
}
