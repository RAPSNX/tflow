// Package buildinfo reports the version of the tflow executable.
package buildinfo

import "runtime/debug"

// version is replaced by release builders with -ldflags -X.
var version = "devel"

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
	if moduleVersion != "" && moduleVersion != "(devel)" {
		return moduleVersion
	}
	return "devel"
}
