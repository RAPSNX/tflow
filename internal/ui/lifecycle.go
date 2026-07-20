package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	runtmux "tflow/internal/tmux"
)

func Start(ctx context.Context) error {
	cwd := defaultSessionDir()
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return startWithManager(ctx, newSessionManager(), exe, cwd, newInstanceID())
}

func startWithManager(ctx context.Context, manager tmuxController, binaryPath, cwd, instanceID string) (result error) {
	if err := os.Setenv(menuInstanceEnv, instanceID); err != nil {
		return err
	}
	sessionName, err := prepareStartup(manager, binaryPath, cwd, instanceID)
	if err != nil {
		return err
	}
	defer func() {
		if cleanupErr := manager.CleanupVolatileSessions(instanceID); cleanupErr != nil && result == nil {
			result = cleanupErr
		}
	}()

	cmd, err := manager.AttachCommand(ctx, sessionName)
	if err != nil {
		return err
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil && isCancellationInducedAttachFailure(err) {
			return nil
		}
		return err
	}
	return nil
}

// isCancellationInducedAttachFailure reports whether err is the kind of
// failure exec.CommandContext produces when it kills the attach process
// because the context was canceled, as opposed to the attach process
// failing on its own for an unrelated, operational reason (for example the
// target tmux session or server disappearing). Only the former should be
// swallowed as part of graceful signal shutdown; per the architecture,
// signal-driven cleanup must never replace another operation's real error.
func isCancellationInducedAttachFailure(err error) bool {
	// exec.CommandContext returns the bare context error (rather than an
	// *exec.ExitError) when the context was already done before the
	// process could even be started.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	// Once the process is running, canceling the context kills it via a
	// signal. That surfaces as an *exec.ExitError whose ProcessState shows
	// the process was terminated by a signal rather than exiting on its
	// own. A process that exits on its own (even with a nonzero status)
	// reports ProcessState.Exited() == true and represents a genuine
	// attach failure that must still be reported.
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && !exitErr.ProcessState.Exited() {
		return true
	}
	return false
}

func OpenMenu(ctx context.Context) error {
	return openMenu(ctx, newSessionManager(), runMenuProgram)
}

type menuProgramRunner func(context.Context, tea.Model) (tea.Model, error)

func openMenu(ctx context.Context, manager tmuxController, runProgram menuProgramRunner) error {
	menu, err := buildModel(manager, os.Getenv(menuCurrentEnv))
	if err != nil {
		return err
	}
	if os.Getenv(runtmux.MenuModeEnv) == runtmux.MenuModeQuit {
		menu.mode = inputConfirmQuit
		menu.status = "Confirm shutdown of this tflow instance."
	}
	finalModel, err := runProgram(ctx, menu)
	if ctx.Err() != nil {
		return nil
	}
	if err != nil {
		return err
	}
	return runMenuExitAction(manager, finalModel)
}

func runMenuProgram(ctx context.Context, menu tea.Model) (tea.Model, error) {
	return tea.NewProgram(menu, tea.WithAltScreen(), tea.WithContext(ctx)).Run()
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
		_ = manager.KillSession(name)
		return "", fmt.Errorf("tag startup session: %w", err)
	}
	if err := manager.SetSessionLabel(name, label); err != nil {
		// Preserve the setup error; deleting the newly created session is best effort.
		_ = manager.KillSession(name)
		return "", fmt.Errorf("set startup session label: %w", err)
	}
	if err := manager.EnsureControlMode(binaryPath); err != nil {
		// Preserve the setup error; deleting the newly created session is best effort.
		_ = manager.KillSession(name)
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
