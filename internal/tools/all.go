// Package tools registra explícitamente todas las herramientas.
package tools

import (
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/converters"
	"github.com/Judamaro/devtooltui-golang/internal/tools/encoders"
	"github.com/Judamaro/devtooltui-golang/internal/tools/formatters"
	"github.com/Judamaro/devtooltui-golang/internal/tools/generators"
)

// All devuelve todas las herramientas en el orden en que se registran.
func All() []tool.Tool {
	return []tool.Tool{
		formatters.JSONFormat{},
		encoders.Base64{},
		encoders.URL{},
		encoders.JWT{},
		converters.Timestamp{},
		converters.NumberBase{},
		converters.JSONYAML{},
		generators.Hash{},
		generators.UUID{},
	}
}
