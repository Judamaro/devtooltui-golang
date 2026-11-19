// Package testutil contiene helpers compartidos por los tests.
package testutil

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "regenera los golden files")

// Updating indica si hay que regenerar los golden files: con -update en un
// paquete concreto o con UPDATE_GOLDEN=1 (lo usa make golden-update, porque
// -update rompe los paquetes que no importan testutil).
func Updating() bool {
	return *update || os.Getenv("UPDATE_GOLDEN") == "1"
}

// Root devuelve la raíz del módulo (el directorio con go.mod).
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
			t.Fatal("no se encontró go.mod")
		}
		dir = parent
	}
}

// Path devuelve testdata/<parts...> relativo a la raíz del módulo.
func Path(t testing.TB, parts ...string) string {
	t.Helper()
	return filepath.Join(append([]string{Root(t), "testdata"}, parts...)...)
}

// ReadInput lee testdata/<id>/<name>.in.
func ReadInput(t testing.TB, id, name string) string {
	t.Helper()
	b, err := os.ReadFile(Path(t, id, name+".in"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Golden compara got con testdata/<id>/<name>.golden, o lo reescribe si
// Updating() es true.
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
		t.Fatalf("%v (ejecuta make golden-update para crearlo)", err)
	}
	if got != string(want) {
		t.Errorf("salida distinta de %s\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
	}
}
