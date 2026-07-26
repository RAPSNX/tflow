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
// optionally followed by build metadata such as +dirty. The toolchain
// auto-generates these for ordinary builds run from a git checkout that is
// ahead of its last tag (or has uncommitted changes), so their presence
// doesn't mean the module version identifies a requested release.
var pseudoVersionSuffix = regexp.MustCompile(`[.-]\d{14}-[0-9a-fA-F]{12}(\+[0-9A-Za-z.-]+)?$`)

func isPseudoVersion(v string) bool {
	return pseudoVersionSuffix.MatchString(v)
}

// Version returns the release version when one is available. Go module builds
// retain their requested module version, while ordinary local builds are
// identified as development builds.
func Version() string {
	moduleVersion := ""
	if info, ok := debug.ReadBuildInfo(); ok {
		moduleVersion = info.Main.Version
	}

	return resolve(version, moduleVersion)
}

func resolve(linkedVersion, moduleVersion string) string {
	if linkedVersion != "" && linkedVersion != "devel" {
		return linkedVersion
	}
	if moduleVersion != "" && moduleVersion != "(devel)" && !isPseudoVersion(moduleVersion) {
		return moduleVersion
	}
	return "devel"
}
