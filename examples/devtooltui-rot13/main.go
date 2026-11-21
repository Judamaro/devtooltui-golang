// Command devtooltui-rot13 is an example devtooltui plugin (protocol 1, see
// docs/plugins.md). Build it into a directory of your PATH and devtooltui
// shows a "ROT13 / ROT47" tool in the TUI and a rot13 subcommand:
//
//	go build -o ~/bin/devtooltui-rot13 ./examples/devtooltui-rot13
//	devtooltui plugin list
//	echo 'Hello' | devtooltui rot13 --variant rot47
//
// It uses only the standard library and its own copies of the messages, as
// a plugin outside this module would: internal/ cannot be imported.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type spec struct {
	Protocol int        `json:"protocol"`
	Tools    []toolSpec `json:"tools"`
}

type toolSpec struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	Description string   `json:"description"`
	Keywords    []string `json:"keywords"`
	Inputs      []input  `json:"inputs"`
	Options     []option `json:"options"`
	Detect      *detect  `json:"detect,omitempty"`
}

type input struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	Multiline bool   `json:"multiline"`
}

type option struct {
	Key     string   `json:"key"`
	Label   string   `json:"label"`
	Help    string   `json:"help,omitempty"`
	Kind    string   `json:"kind"`
	Choices []string `json:"choices,omitempty"`
	Default any      `json:"default"`
}

type detect struct {
	Regex string  `json:"regex"`
	Score float64 `json:"score"`
}

type request struct {
	Protocol int               `json:"protocol"`
	Tool     string            `json:"tool"`
	Inputs   map[string]string `json:"inputs"`
	Options  map[string]any    `json:"options"`
}

type response struct {
	Result *result   `json:"result,omitempty"`
	Error  *inputErr `json:"error,omitempty"`
}

type result struct {
	Text string `json:"text"`
	Meta []kv   `json:"meta,omitempty"`
}

type kv struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type inputErr struct {
	Input   string `json:"input"`
	Line    int    `json:"line"`
	Col     int    `json:"col"`
	Message string `json:"message"`
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: devtooltui-rot13 spec|run (it is run by devtooltui)")
		os.Exit(2)
	}
	if err := handle(os.Args[1], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "devtooltui-rot13:", err)
		os.Exit(1)
	}
}

// handle answers one protocol command.
func handle(cmd string, in io.Reader, out io.Writer) error {
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	switch cmd {
	case "spec":
		return enc.Encode(spec{Protocol: 1, Tools: []toolSpec{{
			ID: "rot13", Name: "ROT13 / ROT47", Category: "Encoders",
			Description: "Rotate letters by 13 (ROT13) or printable ASCII by 47 (ROT47)",
			Keywords:    []string{"caesar", "cipher", "rot47", "spoiler"},
			Inputs:      []input{{Key: "input", Label: "Text", Multiline: true}},
			Options: []option{{Key: "variant", Label: "Variant", Kind: "select",
				Help: "rot13 turns letters only; rot47 every printable ASCII character", Choices: []string{"rot13", "rot47"}, Default: "rot13"}},
		}}})
	case "run":
		var req request
		if err := json.NewDecoder(in).Decode(&req); err != nil {
			return fmt.Errorf("bad request: %w", err)
		}
		return enc.Encode(run(req))
	}
	return fmt.Errorf("unknown command %q", cmd)
}

func run(req request) response {
	text := req.Inputs["input"]
	// Rotation is defined on text: a NUL byte means binary input.
	if i := strings.IndexByte(text, 0); i >= 0 {
		line := 1 + strings.Count(text[:i], "\n")
		col := 1 + len([]rune(text[strings.LastIndexByte(text[:i], '\n')+1:i]))
		return response{Error: &inputErr{Input: "input", Line: line, Col: col, Message: "binary input (NUL byte)"}}
	}
	rot := rot13
	if req.Options["variant"] == "rot47" {
		rot = rot47
	}
	out := strings.Map(rot, text)
	changed := 0
	for i := range len(text) {
		if text[i] != out[i] {
			changed++
		}
	}
	return response{Result: &result{Text: out, Meta: []kv{{Key: "Changed", Value: fmt.Sprint(changed)}}}}
}

func rot13(r rune) rune {
	switch {
	case r >= 'a' && r <= 'z':
		return 'a' + (r-'a'+13)%26
	case r >= 'A' && r <= 'Z':
		return 'A' + (r-'A'+13)%26
	}
	return r
}

func rot47(r rune) rune {
	if r >= '!' && r <= '~' {
		return '!' + (r-'!'+47)%94
	}
	return r
}
