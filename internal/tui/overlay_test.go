package tui

import (
	"testing"

	"github.com/Judamaro/devtooltui-golang/internal/tui/palette"
)

// Overlays stack: closing the top one uncovers the one below, which gets
// the keys again.
func TestOverlayStack(t *testing.T) {
	a := newApp(t, 100, 30)
	send(a, keyCtrl('p'))
	a.openHelp()
	if len(a.overlays) != 2 || !isOpen[*helpOverlay](a) {
		t.Fatalf("overlays = %d, help on top = %v", len(a.overlays), isOpen[*helpOverlay](a))
	}
	press(a, "x") // closes the help only
	if len(a.overlays) != 1 || !isOpen[*palette.Model](a) {
		t.Fatalf("after a key: overlays = %d, palette on top = %v", len(a.overlays), isOpen[*palette.Model](a))
	}
	send(a, keyEsc())
	if len(a.overlays) != 0 {
		t.Errorf("Esc left %d overlays open", len(a.overlays))
	}
}
