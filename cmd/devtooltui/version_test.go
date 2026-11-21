package main

import (
	"runtime/debug"
	"testing"
)

func TestBuildVersion(t *testing.T) {
	setting := func(kv ...string) []debug.BuildSetting {
		var s []debug.BuildSetting
		for i := 0; i+1 < len(kv); i += 2 {
			s = append(s, debug.BuildSetting{Key: kv[i], Value: kv[i+1]})
		}
		return s
	}
	tests := []struct {
		name             string
		version, commit  string
		bi               *debug.BuildInfo
		wantVer, wantCom string
	}{
		{"ldflags win", "1.1.0", "251352e", &debug.BuildInfo{Main: debug.Module{Version: "v9.9.9"}}, "1.1.0", "251352e"},
		{"go install @version", "dev", "none", &debug.BuildInfo{Main: debug.Module{Version: "v1.1.1"}}, "1.1.1", "none"},
		{"checkout build", "dev", "none", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"},
			Settings: setting("vcs.revision", "251352e0123456789", "vcs.modified", "false")}, "dev", "251352e"},
		{"dirty checkout", "dev", "none", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"},
			Settings: setting("vcs.revision", "251352e0123456789", "vcs.modified", "true")}, "dev", "251352e-dirty"},
		{"pseudo-version", "dev", "none", &debug.BuildInfo{Main: debug.Module{Version: "v1.1.1-0.20261121000000-abcdef123456"}}, "1.1.1-0.20261121000000-abcdef123456", "none"},
		{"no build info", "dev", "none", nil, "dev", "none"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, c := buildVersion(tt.version, tt.commit, tt.bi)
			if v != tt.wantVer || c != tt.wantCom {
				t.Errorf("got %q %q, want %q %q", v, c, tt.wantVer, tt.wantCom)
			}
		})
	}
}
