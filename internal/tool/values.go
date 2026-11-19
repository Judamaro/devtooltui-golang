package tool

// Values son los valores de las opciones de una Request, indexados por
// Option.Key. Los getters devuelven el valor cero si falta la clave o el
// tipo no coincide.
type Values map[string]any

func (v Values) Bool(key string) bool {
	b, _ := v[key].(bool)
	return b
}

func (v Values) String(key string) string {
	s, _ := v[key].(string)
	return s
}

func (v Values) Int(key string) int {
	switch n := v[key].(type) {
	case int:
		return n
	case int64:
		return int(n)
	default:
		return 0
	}
}

// WithDefaults devuelve una copia de v completada con los Default de spec
// para las claves que falten.
func (v Values) WithDefaults(spec Spec) Values {
	out := make(Values, len(spec.Options)+len(v))
	for _, o := range spec.Options {
		out[o.Key] = o.Default
	}
	for k, val := range v {
		out[k] = val
	}
	return out
}
