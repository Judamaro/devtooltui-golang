// Package clipboard copia y pega con el portapapeles del sistema (atotto) y
// cae a OSC 52 para copiar por SSH o cuando falta el binario del sistema.
package clipboard

import (
	"errors"
	"io"

	"github.com/atotto/clipboard"
	"github.com/aymanbagabas/go-osc52/v2"
)

// Clipboard es la interfaz que usan la TUI y la CLI.
type Clipboard interface {
	Read() (string, error)
	Write(text string) error
}

// Backend es la parte del sistema que System usa; los tests la reemplazan.
type Backend interface {
	Available() bool
	Read() (string, error)
	Write(text string) error
}

// Local usa los binarios del sistema a través de atotto (pbcopy,
// wl-clipboard, xclip/xsel, clip.exe).
type Local struct{}

func (Local) Available() bool         { return !clipboard.Unsupported }
func (Local) Read() (string, error)   { return clipboard.ReadAll() }
func (Local) Write(text string) error { return clipboard.WriteAll(text) }

// System elige entre el portapapeles local y OSC 52.
type System struct {
	Backend Backend
	Getenv  func(string) string
	// OSC52 recibe la secuencia de escape (la terminal: stderr en la CLI).
	// En la TUI queda nil y se usa tea.SetClipboard.
	OSC52 io.Writer
}

// New arma un System con el portapapeles local.
func New(getenv func(string) string, osc io.Writer) *System {
	return &System{Backend: Local{}, Getenv: getenv, OSC52: osc}
}

// ErrNoOSC52 indica que hay que copiar con OSC 52 pero no hay a dónde
// escribir la secuencia (la TUI lo resuelve con tea.SetClipboard).
var ErrNoOSC52 = errors.New("se necesita OSC 52")

// SSH indica si la sesión es remota: ahí el portapapeles "local" sería el
// del servidor, así que se copia con OSC 52 hacia la terminal del usuario.
func (s *System) SSH() bool {
	return s.Getenv("SSH_TTY") != "" || s.Getenv("SSH_CONNECTION") != ""
}

// UseOSC52 dice si las copias van por OSC 52.
func (s *System) UseOSC52() bool { return s.SSH() || !s.Backend.Available() }

// Write copia text. Por SSH o sin binario local usa OSC 52.
func (s *System) Write(text string) error {
	if !s.UseOSC52() {
		if err := s.Backend.Write(text); err == nil {
			return nil
		}
	}
	if s.OSC52 == nil {
		return ErrNoOSC52
	}
	seq := osc52.New(text)
	switch {
	case s.Getenv("TMUX") != "":
		seq = seq.Tmux()
	case s.Getenv("STY") != "":
		seq = seq.Screen()
	}
	_, err := seq.WriteTo(s.OSC52)
	return err
}

// Read pega desde el portapapeles local. OSC 52 no permite leer de forma
// fiable, así que sin binario local devuelve un error con el paquete a
// instalar.
func (s *System) Read() (string, error) {
	if s.SSH() && !s.Backend.Available() {
		return "", errors.New("por SSH no se puede leer el portapapeles local; pega con el atajo de tu terminal")
	}
	if !s.Backend.Available() {
		return "", errors.New("no hay portapapeles del sistema: " + s.InstallHint())
	}
	return s.Backend.Read()
}

// InstallHint sugiere el paquete que falta en Linux.
func (s *System) InstallHint() string {
	switch {
	case s.Getenv("WAYLAND_DISPLAY") != "":
		return "instala wl-clipboard"
	case s.Getenv("DISPLAY") != "":
		return "instala xclip o xsel"
	default:
		return "instala wl-clipboard (Wayland) o xclip/xsel (X11)"
	}
}
