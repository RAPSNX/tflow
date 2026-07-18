package store

import (
	"os"
	"path/filepath"
	"strings"
)

func NormalizeCWD(cwd string) string {
	cwd = strings.TrimSpace(cwd)
	if cwd == "" {
		if wd, err := os.Getwd(); err == nil {
			cwd = wd
		}
	}
	cwd = expandHomeDir(cwd)
	if cwd == "" {
		return fallbackCWD()
	}
	if abs, err := filepath.Abs(cwd); err == nil {
		return abs
	}
	return fallbackCWD()
}

func fallbackCWD() string {
	for _, path := range []string{userHomeDir(), os.TempDir()} {
		if path == "" || !filepath.IsAbs(path) {
			continue
		}
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			return filepath.Clean(path)
		}
	}
	return string(os.PathSeparator)
}

func userHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(home)
}

func expandHomeDir(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || path[0] != '~' {
		return path
	}
	if len(path) > 1 && path[1] != '/' {
		return path
	}
	home := userHomeDir()
	if home == "" {
		return path
	}
	if path == "~" {
		return home
	}
	return filepath.Join(home, path[2:])
}
