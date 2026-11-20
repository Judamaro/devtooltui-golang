package converters

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/formatters"
)

// CSVJSON converts CSV to a JSON array (of objects with a header row, of
// arrays without) and back.
type CSVJSON struct{}

var csvDelimiters = map[string]rune{"comma": ',', "semicolon": ';', "tab": '\t', "pipe": '|'}

func (CSVJSON) Spec() tool.Spec {
	return tool.Spec{
		ID:          "csv-json",
		Name:        "CSV ↔ JSON",
		Category:    tool.Converters,
		Description: "Convert CSV to a JSON array and back",
		Keywords:    []string{"csv", "tsv", "json", "spreadsheet", "table"},
		Inputs:      []tool.InputSpec{{Key: "input", Label: "Input", Multiline: true}},
		Options: []tool.Option{
			{Key: "mode", Label: "Mode", Kind: tool.Select, Choices: []string{"to-json", "to-csv"}, Default: "to-json"},
			{Key: "delimiter", Label: "Delimiter", Kind: tool.Select, Choices: []string{"comma", "semicolon", "tab", "pipe"}, Default: "comma"},
			{Key: "header", Label: "Header row", Help: "The first CSV row holds the keys of the JSON objects", Kind: tool.Toggle, Default: true},
			{Key: "types", Label: "Infer types", Help: "To JSON: numbers, true/false and empty cells as null", Kind: tool.Toggle, Default: false},
		},
		Output: tool.Highlighted,
		Live:   true,
	}
}

func (c CSVJSON) Run(ctx context.Context, req tool.Request) (tool.Result, error) {
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	opts := req.Opts.WithDefaults(c.Spec())
	in := req.Inputs["input"]
	if strings.TrimSpace(in) == "" {
		return tool.Result{}, nil
	}
	delim := csvDelimiters[opts.String("delimiter")]
	if opts.String("mode") == "to-csv" {
		return jsonToCSV(in, delim, opts.Bool("header"))
	}
	return csvToJSON(ctx, in, delim, opts.Bool("header"), opts.Bool("types"))
}

func csvToJSON(ctx context.Context, in string, delim rune, header, types bool) (tool.Result, error) {
	r := csv.NewReader(strings.NewReader(in))
	r.Comma = delim
	r.FieldsPerRecord = 0 // every row as long as the first
	r.ReuseRecord = false
	var rows [][]string
	for {
		if len(rows)%1024 == 0 {
			if err := ctx.Err(); err != nil {
				return tool.Result{}, err
			}
		}
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return tool.Result{}, csvInputError(in, err)
		}
		rows = append(rows, rec)
	}
	var keys []string
	if header && len(rows) > 0 {
		keys, rows = uniqueKeys(rows[0]), rows[1:]
	}

	var b bytes.Buffer
	b.WriteString("[")
	for i, row := range rows {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("\n  ")
		opener, closer := "[", "]"
		if keys != nil {
			opener, closer = "{", "}"
		}
		b.WriteString(opener)
		for j, cell := range row {
			if j > 0 {
				b.WriteString(", ")
			}
			if keys != nil {
				b.Write(jsonString(keys[j]))
				b.WriteString(": ")
			}
			b.Write(cellJSON(cell, types))
		}
		b.WriteString(closer)
	}
	if len(rows) > 0 {
		b.WriteString("\n")
	}
	b.WriteString("]")
	cols := 0
	if len(rows) > 0 {
		cols = len(rows[0])
	} else if keys != nil {
		cols = len(keys)
	}
	return tool.Result{
		Text: b.String(),
		Lang: "json",
		Meta: []tool.KV{{Key: "Rows", Value: fmt.Sprint(len(rows))}, {Key: "Columns", Value: fmt.Sprint(cols)}},
	}, nil
}

// uniqueKeys names empty headers column_N and suffixes repeated ones. Keys
// are made valid UTF-8 first, as JSON will (two different invalid bytes
// would become the same key).
func uniqueKeys(head []string) []string {
	seen := map[string]int{}
	keys := make([]string, len(head))
	for i, k := range head {
		k = strings.ToValidUTF8(k, "\uFFFD")
		if k == "" {
			k = fmt.Sprintf("column_%d", i+1)
		}
		seen[k]++
		if n := seen[k]; n > 1 {
			k = fmt.Sprintf("%s_%d", k, n)
		}
		keys[i] = k
	}
	return keys
}

func cellJSON(cell string, types bool) []byte {
	if types {
		switch cell {
		case "":
			return []byte("null")
		case "true", "false":
			return []byte(cell)
		}
		if _, err := strconv.ParseFloat(cell, 64); err == nil && json.Valid([]byte(cell)) {
			return []byte(cell) // a JSON number as written (no 1e400 → +Inf)
		}
	}
	return jsonString(cell)
}

func jsonString(s string) []byte {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s) // strings always encode
	return bytes.TrimSuffix(b.Bytes(), []byte("\n"))
}

// csvInputError converts csv.ParseError (byte column) to an InputError.
func csvInputError(in string, err error) error {
	ie := &tool.InputError{Input: "input", Line: 1, Col: 1, Msg: err.Error()}
	var pe *csv.ParseError
	if !errors.As(err, &pe) {
		return ie
	}
	ie.Msg = pe.Err.Error()
	if errors.Is(pe.Err, csv.ErrFieldCount) {
		ie.Msg = "this row has a different number of fields than the first one"
	}
	// Offset of the start of line pe.Line, plus the byte column.
	lineStart := func(n int) int {
		off := 0
		for range n - 1 {
			i := strings.IndexByte(in[off:], '\n')
			if i < 0 {
				break
			}
			off += i + 1
		}
		return off
	}
	off := lineStart(pe.Line)
	lineEnd := len(in)
	if i := strings.IndexByte(in[off:], '\n'); i >= 0 {
		lineEnd = off + i
	}
	if errors.Is(pe.Err, csv.ErrQuote) && off+pe.Column-1 > lineEnd {
		// A quote left open runs past its line: point at the quote that
		// opens it, in the row where the record starts.
		ie.Msg = "quoted field not closed"
		start := lineStart(pe.StartLine)
		q := strings.IndexByte(in[start:], '"')
		ie.Line, ie.Col = tool.Position(in, start+max(q, 0))
		return ie
	}
	ie.Line, ie.Col = tool.Position(in, min(off+max(pe.Column-1, 0), len(in)))
	return ie
}

func jsonToCSV(in string, delim rune, header bool) (tool.Result, error) {
	var discard bytes.Buffer
	if err := json.Compact(&discard, []byte(in)); err != nil {
		return tool.Result{}, formatters.JSONInputError(in, err)
	}
	notTable := &tool.InputError{Input: "input", Line: 1, Col: 1, Msg: "expected a JSON array of objects or of arrays"}
	dec := json.NewDecoder(strings.NewReader(in))
	dec.UseNumber()
	if tok, _ := dec.Token(); tok != json.Delim('[') {
		return tool.Result{}, notTable
	}
	var keys []string
	keyIndex := map[string]int{}
	var objects []map[string]json.RawMessage
	var arrays [][]json.RawMessage
	for i := 0; dec.More(); i++ {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return tool.Result{}, formatters.JSONInputError(in, err)
		}
		switch first := firstByte(raw); {
		case first == '{' && arrays == nil:
			obj, order, err := orderedObject(raw)
			if err != nil {
				return tool.Result{}, formatters.JSONInputError(in, err)
			}
			for _, k := range order {
				if _, ok := keyIndex[k]; !ok {
					keyIndex[k] = len(keys)
					keys = append(keys, k)
				}
			}
			objects = append(objects, obj)
		case first == '[' && objects == nil:
			var row []json.RawMessage
			if err := json.Unmarshal(raw, &row); err != nil {
				return tool.Result{}, formatters.JSONInputError(in, err)
			}
			arrays = append(arrays, row)
		default:
			notTable.Msg = fmt.Sprintf("element %d: every element must be an object, or every element an array", i)
			return tool.Result{}, notTable
		}
	}

	var b bytes.Buffer
	w := csv.NewWriter(&b)
	w.Comma = delim
	write := func(rec []string) {
		if len(rec) == 1 && rec[0] == "" {
			// csv.Writer would write an empty line, which readers skip.
			w.Flush()
			b.WriteString("\"\"\n")
			return
		}
		_ = w.Write(rec) // a bytes.Buffer never fails
	}
	if objects != nil {
		if header {
			write(keys)
		}
		for _, obj := range objects {
			rec := make([]string, len(keys))
			for k, v := range obj {
				rec[keyIndex[k]] = cellText(v)
			}
			write(rec)
		}
	}
	for _, row := range arrays {
		rec := make([]string, len(row))
		for j, v := range row {
			rec[j] = cellText(v)
		}
		write(rec)
	}
	w.Flush()
	rows := len(objects) + len(arrays)
	return tool.Result{
		Text: strings.TrimSuffix(b.String(), "\n"),
		Meta: []tool.KV{{Key: "Rows", Value: fmt.Sprint(rows)}, {Key: "Columns", Value: fmt.Sprint(len(keys))}},
	}, nil
}

func firstByte(raw json.RawMessage) byte {
	t := bytes.TrimLeft(raw, " \t\r\n")
	if len(t) == 0 {
		return 0
	}
	return t[0]
}

// orderedObject decodes one object keeping its key order.
func orderedObject(raw json.RawMessage) (map[string]json.RawMessage, []string, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	if _, err := dec.Token(); err != nil { // {
		return nil, nil, err
	}
	obj := map[string]json.RawMessage{}
	var order []string
	for dec.More() {
		k, err := dec.Token()
		if err != nil {
			return nil, nil, err
		}
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, nil, err
		}
		key := k.(string)
		if _, dup := obj[key]; !dup {
			order = append(order, key)
		}
		obj[key] = v
	}
	return obj, order, nil
}

// cellText: strings unquoted, null empty, anything else as compact JSON.
func cellText(v json.RawMessage) string {
	var s string
	if json.Unmarshal(v, &s) == nil {
		return s
	}
	if string(bytes.TrimSpace(v)) == "null" {
		return ""
	}
	var b bytes.Buffer
	if json.Compact(&b, v) != nil {
		return string(v)
	}
	return b.String()
}
