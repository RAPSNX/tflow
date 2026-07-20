package ui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"tflow/internal/diag"
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
		// Preserve the attach error; volatile cleanup is deliberately best effort.
		if cleanupErr := manager.CleanupVolatileSessions(instanceID); cleanupErr != nil {
			diag.Warnf("clean up volatile sessions for instance %q after attach failure: %v", instanceID, cleanupErr)
		}
		return err
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	runErr := cmd.Run()
	cleanupErr := manager.CleanupVolatileSessions(instanceID)
	if runErr != nil {
		// Preserve the client error; volatile cleanup is deliberately best effort.
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
	path := appStatePath()
	if _, err := loadAppState(path); err != nil {
		return "", fmt.Errorf("load state %q: %w", path, err)
	}

	var existing []session
	if _, err := reconcileAppState(path, func() (map[string]struct{}, error) {
		var err error
		existing, err = manager.ListSessions()
		if err != nil {
			return nil, err
		}
		existingNames := make(map[string]struct{}, len(existing))
		for _, session := range existing {
			existingNames[session.Name] = struct{}{}
		}
		return existingNames, nil
	}); err != nil {
		return "", fmt.Errorf("reconcile state %q: %w", path, err)
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
		// Preserve the setup error; deleting the newly created session is best effort.
		if killErr := manager.KillSession(name); killErr != nil {
			diag.Warnf("kill orphaned startup session %q after tagging failure: %v", name, killErr)
		}
		return "", fmt.Errorf("tag startup session: %w", err)
	}
	if err := manager.SetSessionLabel(name, label); err != nil {
		// Preserve the setup error; deleting the newly created session is best effort.
		if killErr := manager.KillSession(name); killErr != nil {
			diag.Warnf("kill orphaned startup session %q after label failure: %v", name, killErr)
		}
		return "", fmt.Errorf("set startup session label: %w", err)
	}
	if err := manager.EnsureControlMode(binaryPath); err != nil {
		// Preserve the setup error; deleting the newly created session is best effort.
		if killErr := manager.KillSession(name); killErr != nil {
			diag.Warnf("kill orphaned startup session %q after control-mode failure: %v", name, killErr)
		}
		return "", fmt.Errorf("prepare tmux control mode: %w", err)
	}
	return name, nil
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
