package buildinfo

import (
	"runtime/debug"
	"testing"
)

func TestResolve(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		settings []debug.BuildSetting
		want     string
	}{
		{
			name: "release wins over revision", version: "0.9.0",
			settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "abc123"}}, want: "0.9.0",
		},
		{
			name: "explicit dev wins over revision", version: "dev",
			settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "abc123"}}, want: "dev",
		},
		{
			name: "release without build settings", version: "0.9.0", want: "0.9.0",
		},
		{
			name:     "revision fallback",
			settings: []debug.BuildSetting{{Key: "vcs.modified", Value: "true"}, {Key: "vcs.revision", Value: "abc123"}},
			want:     "abc123",
		},
		{
			name: "no build settings", want: "dev",
		},
		{
			name:     "unrelated build settings",
			settings: []debug.BuildSetting{{Key: "vcs.time", Value: "2026-11-02T00:00:00Z"}}, want: "dev",
		},
		{
			name: "empty revision", settings: []debug.BuildSetting{{Key: "vcs.revision"}}, want: "dev",
		},
		{
			name:     "empty revision does not mask a later value",
			settings: []debug.BuildSetting{{Key: "vcs.revision"}, {Key: "vcs.revision", Value: "abc123"}}, want: "abc123",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolve(tt.version, tt.settings); got != tt.want {
				t.Errorf("resolve(%q, %+v) = %q, want %q", tt.version, tt.settings, got, tt.want)
			}
		})
	}
}
