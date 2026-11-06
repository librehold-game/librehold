// Package buildinfo reports the release version or revision of the running binary.
package buildinfo

import "runtime/debug"

// version is set at build time with -X librehold/internal/buildinfo.version=0.9.0.
var version string

// Version is the release version, else the git revision, else "dev".
func Version() string {
	var settings []debug.BuildSetting
	if info, ok := debug.ReadBuildInfo(); ok {
		settings = info.Settings
	}
	return resolve(version, settings)
}

func resolve(version string, settings []debug.BuildSetting) string {
	if version != "" {
		return version
	}
	for _, setting := range settings {
		if setting.Key == "vcs.revision" && setting.Value != "" {
			return setting.Value
		}
	}
	return "dev"
}
