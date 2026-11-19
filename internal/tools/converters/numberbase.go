package converters

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// NumberBase convierte enteros de cualquier tamaño entre bases 2, 8, 10 y 16.
type NumberBase struct{}

func (NumberBase) Spec() tool.Spec {
	return tool.Spec{
		ID:          "number-base",
		Name:        "Bases numéricas",
		Category:    tool.Converters,
		Description: "Convierte enteros entre binario, octal, decimal y hexadecimal",
		Keywords:    []string{"binario", "hex", "octal", "decimal", "radix", "base"},
		Inputs:      []tool.InputSpec{{Key: "input", Label: "Número"}},
		Options: []tool.Option{
			{Key: "from", Label: "Base de entrada", Help: "auto: por prefijo 0x, 0b, 0o; si no, decimal", Kind: tool.Select,
				Choices: []string{"auto", "2", "8", "10", "16"}, Default: "auto"},
			{Key: "group", Label: "Agrupar", Help: "Separa dígitos con '_'", Kind: tool.Toggle, Default: true},
		},
		Output: tool.PlainText,
		Live:   true,
	}
}

var basePrefixes = map[int]string{2: "0b", 8: "0o", 10: "", 16: "0x"}

func (nb NumberBase) Run(ctx context.Context, req tool.Request) (tool.Result, error) {
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	opts := req.Opts.WithDefaults(nb.Spec())
	in := req.Inputs["input"]
	if strings.TrimSpace(in) == "" {
		return tool.Result{}, nil
	}
	n, base, err := parseInt(in, opts.String("from"))
	if err != nil {
		return tool.Result{}, err
	}
	group := opts.Bool("group")
	rows := []struct {
		label string
		base  int
	}{{"Binario", 2}, {"Octal", 8}, {"Decimal", 10}, {"Hexadecimal", 16}}
	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b, "%-12s %s\n", r.label+":", formatInt(n, r.base, group))
	}

	fits := "no cabe en 64 bits"
	switch {
	case n.IsInt64():
		fits = "cabe en int64"
	case n.IsUint64():
		fits = "cabe en uint64"
	}
	return tool.Result{
		Text: strings.TrimRight(b.String(), "\n"),
		Meta: []tool.KV{
			{Key: "Base de entrada", Value: strconv.Itoa(base)},
			{Key: "Bits", Value: strconv.Itoa(n.BitLen())},
			{Key: "Rango", Value: fits},
		},
	}, nil
}

// parseInt acepta signo, prefijo (en auto) y separadores '_' o espacio, y
// señala la columna del primer dígito inválido.
func parseInt(in, from string) (*big.Int, int, error) {
	start := len(in) - len(strings.TrimLeft(in, " \t\r\n"))
	s := strings.TrimSpace(in)
	pos := start // offset en in del carácter actual de s

	neg := false
	if s != "" && (s[0] == '-' || s[0] == '+') {
		neg = s[0] == '-'
		s, pos = s[1:], pos+1
	}
	base := 10
	if from != "auto" {
		base, _ = strconv.Atoi(from)
	}
	if len(s) > 2 && s[0] == '0' {
		if b, ok := map[byte]int{'x': 16, 'X': 16, 'b': 2, 'B': 2, 'o': 8, 'O': 8}[s[1]]; ok && (from == "auto" || b == base) {
			base = b
			s, pos = s[2:], pos+2
		}
	}

	var digits strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '_' || c == ' ' {
			continue
		}
		if v := digitValue(c); v < 0 || v >= base {
			line, col := tool.Position(in, pos+i)
			return nil, base, &tool.InputError{Input: "input", Line: line, Col: col,
				Msg: fmt.Sprintf("%q no es un dígito válido en base %d", c, base)}
		}
		digits.WriteByte(c)
	}
	if digits.Len() == 0 {
		line, col := tool.Position(in, pos)
		return nil, base, &tool.InputError{Input: "input", Line: line, Col: col, Msg: "faltan dígitos"}
	}
	n, _ := new(big.Int).SetString(digits.String(), base) // ya validado
	if neg {
		n.Neg(n)
	}
	return n, base, nil
}

func digitValue(c byte) int {
	switch {
	case '0' <= c && c <= '9':
		return int(c - '0')
	case 'a' <= c && c <= 'f':
		return int(c-'a') + 10
	case 'A' <= c && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

func formatInt(n *big.Int, base int, group bool) string {
	digits := new(big.Int).Abs(n).Text(base)
	if group {
		size := 3
		if base == 2 || base == 16 {
			size = 4
		}
		digits = groupDigits(digits, size)
	}
	sign := ""
	if n.Sign() < 0 {
		sign = "-"
	}
	return sign + basePrefixes[base] + digits
}

// groupDigits separa con '_' cada size dígitos desde la derecha.
func groupDigits(s string, size int) string {
	if len(s) <= size {
		return s
	}
	var b strings.Builder
	head := len(s) % size
	if head > 0 {
		b.WriteString(s[:head])
	}
	for i := head; i < len(s); i += size {
		if b.Len() > 0 {
			b.WriteByte('_')
		}
		b.WriteString(s[i : i+size])
	}
	return b.String()
}
