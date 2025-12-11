package version

import (
	"runtime/debug"
)

// Version is set via ldflags during release builds.
// When not set (i.e., "dev"), we fall back to VCS info from go build.
var Version = "dev"

func init() {
	if Version != "dev" {
		return
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}

	var revision, modified string
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value
		}
	}

	if revision != "" {
		// Use short commit hash
		if len(revision) > 7 {
			revision = revision[:7]
		}
		Version = "dev-" + revision
		if modified == "true" {
			Version += "-dirty"
		}
	}
}
