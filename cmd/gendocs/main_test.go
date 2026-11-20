package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Judamaro/devtooltui-golang/internal/testutil"
)

// TestSiteUpToDate regenerates the site and compares it with docs/: a new
// tool, option or golden without `make docs` fails here. UPDATE_GOLDEN=1
// (make golden-update) rewrites docs/.
func TestSiteUpToDate(t *testing.T) {
	root := testutil.Root(t)
	testdata := filepath.Join(root, "testdata")
	docs := filepath.Join(root, "docs")
	if testutil.Updating() {
		if err := run(docs, "", testdata); err != nil {
			t.Fatal(err)
		}
	}
	tmp := t.TempDir()
	if err := run(tmp, "", testdata); err != nil {
		t.Fatal(err)
	}
	err := filepath.WalkDir(tmp, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(tmp, path)
		want, _ := os.ReadFile(path)
		got, rerr := os.ReadFile(filepath.Join(docs, rel)) //nolint:gosec // test files
		if rerr != nil || string(got) != string(want) {
			t.Errorf("docs/%s is out of date: run make docs", filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Pages of tools that no longer exist.
	entries, _ := os.ReadDir(filepath.Join(docs, "tools"))
	for _, e := range entries {
		if _, err := os.Stat(filepath.Join(tmp, "tools", e.Name())); err != nil {
			t.Errorf("docs/tools/%s belongs to no tool: delete it", e.Name())
		}
	}
}

func TestManPages(t *testing.T) {
	dir := t.TempDir()
	if err := run("", dir, filepath.Join(testutil.Root(t), "testdata")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"devtooltui.1", "devtooltui-json-format.1", "devtooltui-sql-format.1", "devtooltui-list.1"} {
		data, err := os.ReadFile(filepath.Join(dir, name)) //nolint:gosec // test files
		if err != nil {
			t.Errorf("missing %s: %v", name, err)
			continue
		}
		if !strings.Contains(string(data), ".TH \"DEVTOOLTUI\"") {
			t.Errorf("%s has no man header", name)
		}
	}
	page, _ := os.ReadFile(filepath.Join(dir, "devtooltui-json-format.1")) //nolint:gosec // test files
	if !strings.Contains(string(page), "--sort-keys") {
		t.Error("the json-format page does not list --sort-keys")
	}
}
