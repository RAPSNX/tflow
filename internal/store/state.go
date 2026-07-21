package store

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/rapsnx/tflow/internal/diag"
)

var (
	writeAppStateTemporary = func(file *os.File, data []byte) (int, error) {
		return file.Write(data)
	}
	renameAppStateFile = os.Rename
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
	if err := os.Chmod(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".store.json-")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	closed := false
	written := false
	defer func() {
		if !closed {
			if err := temporary.Close(); err != nil {
				diag.Warnf("close abandoned app-state temp file %q: %v", temporaryPath, err)
			}
		}
		if !written {
			if err := os.Remove(temporaryPath); err != nil && !os.IsNotExist(err) {
				diag.Warnf("remove abandoned app-state temp file %q: %v", temporaryPath, err)
			}
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return err
	}
	if _, err := writeAppStateTemporary(temporary, data); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	closed = true
	if err := renameAppStateFile(temporaryPath, path); err != nil {
		return err
	}
	written = true
	return nil
}

func EnsureStartupState() error {
	path := AppStatePath()
	_, err := MutateAppState(path, func(state AppState) (AppState, error) {
		return state, nil
	})
	return err
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
