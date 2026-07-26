// Package buildinfo reports the version of the tflow executable.
package buildinfo

import (
	"regexp"
	"runtime/debug"
)

// version is replaced by release builders with -ldflags -X.
var version = "devel"

// pseudoVersionSuffix matches the suffix Go stamps onto a module version that
// doesn't resolve to an exact tag, e.g. v0.1.0-alpha.1.0.20260726141959-8abb593b2a90,
// optionally followed by build metadata such as +dirty.
var pseudoVersionSuffix = regexp.MustCompile(`[.-]\d{14}-[0-9a-fA-F]{12}(\+[0-9A-Za-z.-]+)?$`)

func isPseudoVersion(v string) bool {
	return pseudoVersionSuffix.MatchString(v)
}

// Version returns the release version when one is available. Go module builds
// retain their requested module version, while ordinary local builds are
// identified as development builds.
func Version() string {
	moduleVersion := ""
	localVCSBuild := false
	if info, ok := debug.ReadBuildInfo(); ok {
		moduleVersion = info.Main.Version
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				localVCSBuild = true
				break
			}
		}
	}

	return resolve(version, moduleVersion, localVCSBuild)
}

// resolve reports the release version when one is available. localVCSBuild
// distinguishes an ordinary local build compiled directly from a git
// checkout -- which carries vcs.revision build settings and can have its
// module version auto-stamped with a pseudo-version by the toolchain purely
// from local VCS state -- from `go install pkg@<commit-or-branch>`, where the
// resulting pseudo-version is fetched from the module proxy and does
// genuinely identify the requested module revision. Only the former should
// fall back to a development version; the latter is honored as-is.
func resolve(linkedVersion, moduleVersion string, localVCSBuild bool) string {
	if linkedVersion != "" && linkedVersion != "devel" {
		return linkedVersion
	}
	if moduleVersion != "" && moduleVersion != "(devel)" && !(localVCSBuild && isPseudoVersion(moduleVersion)) {
		return moduleVersion
	}
	return "devel"
}
