package clipboard_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/Judamaro/devtooltui-golang/internal/clipboard"
)

type fakeBackend struct {
	available bool
	failWrite bool
	data      string
}

func (f *fakeBackend) Available() bool { return f.available }
func (f *fakeBackend) Read() (string, error) {
	return f.data, nil
}
func (f *fakeBackend) Write(s string) error {
	if f.failWrite {
		return errors.New("falló")
	}
	f.data = s
	return nil
}

func env(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

func TestWrite(t *testing.T) {
	tests := []struct {
		name      string
		backend   fakeBackend
		env       map[string]string
		noOSC     bool
		wantLocal bool
		wantOSC   string // prefijo de la secuencia escrita
		wantErr   error
	}{
		{"local", fakeBackend{available: true}, nil, false, true, "", nil},
		{"ssh usa osc52", fakeBackend{available: true}, map[string]string{"SSH_TTY": "/dev/pts/1"}, false, false, "\x1b]52;c;", nil},
		{"sin binario usa osc52", fakeBackend{}, nil, false, false, "\x1b]52;c;", nil},
		{"falla local cae a osc52", fakeBackend{available: true, failWrite: true}, nil, false, false, "\x1b]52;c;", nil},
		{"tmux envuelve", fakeBackend{}, map[string]string{"TMUX": "1"}, false, false, "\x1bPtmux;", nil},
		{"sin destino osc52", fakeBackend{}, nil, true, false, "", clipboard.ErrNoOSC52},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			s := &clipboard.System{Backend: &tt.backend, Getenv: env(tt.env), OSC52: &out}
			if tt.noOSC {
				s.OSC52 = nil
			}
			err := s.Write("hola")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if (tt.backend.data == "hola") != tt.wantLocal {
				t.Errorf("copia local = %v, want %v", tt.backend.data == "hola", tt.wantLocal)
			}
			if !strings.HasPrefix(out.String(), tt.wantOSC) || (tt.wantOSC == "" && out.Len() > 0) {
				t.Errorf("OSC52 = %q, want prefijo %q", out.String(), tt.wantOSC)
			}
		})
	}
}

func TestRead(t *testing.T) {
	tests := []struct {
		name    string
		backend fakeBackend
		env     map[string]string
		want    string
		errHas  string
	}{
		{"local", fakeBackend{available: true, data: "x"}, nil, "x", ""},
		{"wayland sin binario", fakeBackend{}, map[string]string{"WAYLAND_DISPLAY": "wayland-0"}, "", "wl-clipboard"},
		{"x11 sin binario", fakeBackend{}, map[string]string{"DISPLAY": ":0"}, "", "xclip o xsel"},
		{"ssh sin binario", fakeBackend{}, map[string]string{"SSH_CONNECTION": "1"}, "", "por SSH"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &clipboard.System{Backend: &tt.backend, Getenv: env(tt.env)}
			got, err := s.Read()
			if got != tt.want || (tt.errHas == "") != (err == nil) || (err != nil && !strings.Contains(err.Error(), tt.errHas)) {
				t.Errorf("Read() = %q, %v; want %q, error con %q", got, err, tt.want, tt.errHas)
			}
		})
	}
}
