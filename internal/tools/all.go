// Package tools registers every tool explicitly.
package tools

import (
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/converters"
	"github.com/Judamaro/devtooltui-golang/internal/tools/encoders"
	"github.com/Judamaro/devtooltui-golang/internal/tools/formatters"
	"github.com/Judamaro/devtooltui-golang/internal/tools/generators"
	"github.com/Judamaro/devtooltui-golang/internal/tools/testers"
	"github.com/Judamaro/devtooltui-golang/internal/tools/text"
)

// All returns every tool in registration order.
func All() []tool.Tool {
	return []tool.Tool{
		formatters.JSONFormat{},
		formatters.XMLFormat{},
		encoders.Base64{},
		encoders.URL{},
		encoders.JWT{},
		converters.Timestamp{},
		converters.NumberBase{},
		converters.JSONYAML{},
		generators.Hash{},
		generators.UUID{},
		testers.Regex{},
		text.Diff{},
		encoders.HTML{},
		encoders.Gzip{},
		generators.Password{},
		generators.Lorem{},
	}
}
