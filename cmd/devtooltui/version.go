package main

import (
	"runtime/debug"
	"strings"
)

// buildVersion completes version and commit when -ldflags did not set
// them (go install, go build): go install …@v1.1.0 records the module
// version, and a build from a checkout records the VCS revision.
func buildVersion(version, commit string, bi *debug.BuildInfo) (string, string) {
	if bi == nil {
		return version, commit
	}
	if version == "dev" {
		if v := bi.Main.Version; v != "" && v != "(devel)" {
			version = strings.TrimPrefix(v, "v")
		}
	}
	if commit == "none" {
		for _, s := range bi.Settings {
			if s.Key == "vcs.revision" && len(s.Value) >= 7 {
				commit = s.Value[:7]
			}
		}
		for _, s := range bi.Settings {
			if s.Key == "vcs.modified" && s.Value == "true" && commit != "none" {
				commit += "-dirty"
			}
		}
	}
	return version, commit
}
