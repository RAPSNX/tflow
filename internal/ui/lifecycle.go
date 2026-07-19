package ui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	runtmux "tflow/internal/tmux"
)

func Start() error {
	cwd := defaultSessionDir()
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return startWithManager(newSessionManager(), exe, cwd, newInstanceID())
}

func startWithManager(manager tmuxController, binaryPath, cwd, instanceID string) error {
	if err := os.Setenv(menuInstanceEnv, instanceID); err != nil {
		return err
	}
	sessionName, err := prepareStartup(manager, binaryPath, cwd, instanceID)
	if err != nil {
		return err
	}

	cmd, err := manager.AttachCommand(sessionName)
	if err != nil {
		cleanupErr := manager.CleanupVolatileSessions(instanceID)
		if cleanupErr != nil {
			return fmt.Errorf("attach command: %w; cleanup volatile sessions: %v", err, cleanupErr)
		}
		return err
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	runErr := cmd.Run()
	cleanupErr := manager.CleanupVolatileSessions(instanceID)
	if runErr != nil {
		if cleanupErr != nil {
			return fmt.Errorf("attach session: %w; cleanup volatile sessions: %v", runErr, cleanupErr)
		}
		return runErr
	}
	return cleanupErr
}

func OpenMenu() error {
	menu, err := buildModel(newSessionManager(), os.Getenv(menuCurrentEnv))
	if err != nil {
		return err
	}
	if os.Getenv(runtmux.MenuModeEnv) == runtmux.MenuModeQuit {
		menu.mode = inputConfirmQuit
		menu.status = "Confirm shutdown of this tflow instance."
	}
	finalModel, err := tea.NewProgram(menu, tea.WithAltScreen()).Run()
	if err != nil {
		return err
	}
	return runMenuExitAction(newSessionManager(), finalModel)
}

func prepareStartup(manager tmuxController, binaryPath, cwd, instanceID string) (string, error) {
	if err := ensureStartupState(); err != nil {
		return "", fmt.Errorf("initialize state %q: %w", appStatePath(), err)
	}

	existing, err := manager.ListSessions()
	if err != nil {
		return "", err
	}
	label := nextTempSessionNameForInstance(existing, instanceID)
	var name string
	created := false
	for attempts := 0; attempts < startupSessionRetryLimit; attempts++ {
		id, err := newSessionID()
		if err != nil {
			return "", fmt.Errorf("generate startup session id: %w", err)
		}
		name = volatileSessionName(instanceID, id)
		if _, err := manager.CreateSession(name, cwd, ""); err != nil {
			if !isSessionExists(err) {
				return "", err
			}
			continue
		}
		created = true
		break
	}
	if !created {
		return "", fmt.Errorf("allocate startup session after %d retries", startupSessionRetryLimit)
	}
	if err := manager.SetSessionTemporary(name, true, instanceID); err != nil {
		return "", rollbackStartupSession(manager, name, fmt.Errorf("tag startup session: %w", err))
	}
	if err := manager.SetSessionLabel(name, label); err != nil {
		return "", rollbackStartupSession(manager, name, fmt.Errorf("set startup session label: %w", err))
	}
	if err := manager.EnsureControlMode(binaryPath); err != nil {
		return "", rollbackStartupSession(manager, name, fmt.Errorf("prepare tmux control mode: %w", err))
	}
	return name, nil
}

func rollbackStartupSession(manager tmuxController, name string, startupErr error) error {
	if err := manager.KillSession(name); err != nil {
		return fmt.Errorf("%w; rollback startup session %q: %v", startupErr, name, err)
	}
	return startupErr
}

func defaultSessionDir() string {
	if cwd, err := os.Getwd(); err == nil && strings.TrimSpace(cwd) != "" {
		return cwd
	}
	return runtmux.NormalizeCWD("")
}

func runMenuExitAction(manager tmuxController, final tea.Model) error {
	menu, ok := unwrapMenuModel(final)
	if !ok {
		return nil
	}
	switch menu.exitAction {
	case menuExitSwitchSession:
		if strings.TrimSpace(menu.exitSessionName) == "" {
			return nil
		}
		return manager.SwitchClient(menu.exitSessionName)
	case menuExitQuit:
		return manager.QuitAll()
	default:
		return nil
	}
}

func unwrapMenuModel(value tea.Model) (model, bool) {
	switch typed := value.(type) {
	case model:
		return typed, true
	case *model:
		if typed == nil {
			return model{}, false
		}
		return *typed, true
	default:
		return model{}, false
	}
}
