// Package testutil holds helpers shared by the tests.
package testutil

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	// Defines the -update flag; teatest uses it too, so it cannot be
	// declared twice in the same test binary.
	_ "github.com/charmbracelet/x/exp/golden"
)

// Updating reports whether golden files must be regenerated: with -update in
// a single package or with UPDATE_GOLDEN=1 (used by make golden-update,
// because -update breaks packages that do not import testutil).
func Updating() bool {
	if f := flag.Lookup("update"); f != nil && f.Value.String() == "true" {
		return true
	}
	return os.Getenv("UPDATE_GOLDEN") == "1"
}

// Root returns the module root (the directory holding go.mod).
func Root(t testing.TB) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

// Path returns testdata/<parts...> relative to the module root.
func Path(t testing.TB, parts ...string) string {
	t.Helper()
	return filepath.Join(append([]string{Root(t), "testdata"}, parts...)...)
}

// ReadInput reads testdata/<id>/<name>.in.
func ReadInput(t testing.TB, id, name string) string {
	t.Helper()
	b, err := os.ReadFile(Path(t, id, name+".in"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Golden compares got with testdata/<id>/<name>.golden, or rewrites it when
// Updating() is true.
func Golden(t testing.TB, id, name, got string) {
	t.Helper()
	path := Path(t, id, name+".golden")
	if Updating() {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run make golden-update to create it)", err)
	}
	if got != string(want) {
		t.Errorf("output differs from %s\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
	}
}
