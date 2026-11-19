// Package tools registra explícitamente todas las herramientas.
package tools

import (
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/encoders"
	"github.com/Judamaro/devtooltui-golang/internal/tools/formatters"
)

// All devuelve todas las herramientas en el orden en que se registran.
func All() []tool.Tool {
	return []tool.Tool{
		formatters.JSONFormat{},
		encoders.Base64{},
	}
}
