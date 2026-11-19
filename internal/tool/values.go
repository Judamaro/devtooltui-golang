package tool

// Values holds the option values of a Request, keyed by Option.Key. The
// getters return the zero value when the key is missing or the type does
// not match.
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

// WithDefaults returns a copy of v filled in with spec's Default values for
// the missing keys.
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
