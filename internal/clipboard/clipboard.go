// Package clipboard copies and pastes with the system clipboard (atotto) and
// falls back to OSC 52 for copying over SSH or when the system binary is missing.
package clipboard

import (
	"errors"
	"io"

	"github.com/atotto/clipboard"
	"github.com/aymanbagabas/go-osc52/v2"
)

// Clipboard is the interface used by the TUI and the CLI.
type Clipboard interface {
	Read() (string, error)
	Write(text string) error
}

// Backend is the system part System relies on; tests replace it.
type Backend interface {
	Available() bool
	Read() (string, error)
	Write(text string) error
}

// Local uses the system binaries through atotto (pbcopy,
// wl-clipboard, xclip/xsel, clip.exe).
type Local struct{}

func (Local) Available() bool         { return !clipboard.Unsupported }
func (Local) Read() (string, error)   { return clipboard.ReadAll() }
func (Local) Write(text string) error { return clipboard.WriteAll(text) }

// System chooses between the local clipboard and OSC 52.
type System struct {
	Backend Backend
	Getenv  func(string) string
	// OSC52 receives the escape sequence (the terminal: stderr in the CLI).
	// In the TUI it stays nil and tea.SetClipboard is used instead.
	OSC52 io.Writer
}

// New builds a System backed by the local clipboard.
func New(getenv func(string) string, osc io.Writer) *System {
	return &System{Backend: Local{}, Getenv: getenv, OSC52: osc}
}

// ErrNoOSC52 means the copy must go through OSC 52 but there is nowhere to
// write the sequence (the TUI handles it with tea.SetClipboard).
var ErrNoOSC52 = errors.New("OSC 52 required")

// SSH reports whether the session is remote: there the "local" clipboard
// would be the server's, so copies go through OSC 52 to the user's terminal.
func (s *System) SSH() bool {
	return s.Getenv("SSH_TTY") != "" || s.Getenv("SSH_CONNECTION") != ""
}

// UseOSC52 reports whether copies go through OSC 52.
func (s *System) UseOSC52() bool { return s.SSH() || !s.Backend.Available() }

// Write copies text. Over SSH or without a local binary it uses OSC 52.
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

// Read pastes from the local clipboard. OSC 52 cannot read reliably, so
// without a local binary it returns an error naming the package to
// install.
func (s *System) Read() (string, error) {
	if s.SSH() && !s.Backend.Available() {
		return "", errors.New("the local clipboard cannot be read over SSH; paste with your terminal's shortcut")
	}
	if !s.Backend.Available() {
		return "", errors.New("no system clipboard: " + s.InstallHint())
	}
	return s.Backend.Read()
}

// InstallHint suggests the missing package on Linux.
func (s *System) InstallHint() string {
	switch {
	case s.Getenv("WAYLAND_DISPLAY") != "":
		return "install wl-clipboard"
	case s.Getenv("DISPLAY") != "":
		return "install xclip or xsel"
	default:
		return "install wl-clipboard (Wayland) or xclip/xsel (X11)"
	}
}
