package store

import (
	"os"
	"path/filepath"
	"strings"
)

func AppStatePath() string {
	return filepath.Join(stateHomeDir(), "tflow", "store.json")
}

func LoadAppState(path string) (AppState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return emptyAppState(), nil
		}
		return AppState{}, err
	}
	return decodeAppState(data)
}

func SaveAppState(path string, state AppState) error {
	data, err := encodeAppState(NormalizeAppState(state))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func EnsureStartupState() error {
	path := AppStatePath()
	state, err := LoadAppState(path)
	if err != nil {
		return err
	}
	return SaveAppState(path, state)
}

func stateHomeDir() string {
	if dir := strings.TrimSpace(os.Getenv("XDG_STATE_HOME")); dir != "" {
		return dir
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, ".local", "state")
	}
	return filepath.Join(".", ".local", "state")
}
