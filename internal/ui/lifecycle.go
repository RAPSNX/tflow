package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

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
	return startWithManager(newSessionManager(), exe, cwd, newInstanceID(), signalContext)
}

// startWithManager runs startup, then attaches to the resulting session.
// newAttachContext builds the context used for the attach boundary; it is
// called only once startup has produced a session to attach to, not any
// earlier. prepareStartup and the tmux setup calls it makes are
// synchronous and not context-aware, so installing a signal-driven context
// before they run would disable the Go runtime's default terminate-on-
// signal behavior for the whole process while nothing is watching that
// context yet -- a SIGTERM arriving while startup is stuck in one of those
// calls would then be silently swallowed instead of terminating tflow.
func startWithManager(manager tmuxController, binaryPath, cwd, instanceID string, newAttachContext func() (context.Context, context.CancelFunc)) (result error) {
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

	// Signal interception begins here, at the attach boundary, not before
	// prepareStartup.
	ctx, stop := newAttachContext()
	defer stop()

	cmd, err := manager.AttachCommand(ctx, sessionName)
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
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil && isCancellationInducedAttachFailure(err) {
			return nil
		}
		// Preserve the client error; volatile cleanup is deliberately best effort.
		if cleanupErr := manager.CleanupVolatileSessions(instanceID); cleanupErr != nil {
			diag.Warnf("clean up volatile sessions for instance %q after client error: %v", instanceID, cleanupErr)
		}
		return err
	}
	return nil
}

// signalContext creates the runtime context canceled by SIGHUP, SIGINT, and
// SIGTERM for the attached tmux client. See startWithManager for why this
// is constructed lazily at the attach boundary rather than at process
// startup.
func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGHUP, syscall.SIGTERM)
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
	if ctx.Err() != nil && isCancellationInducedPopupExit(ctx, err) {
		return nil
	}
	if err != nil {
		return err
	}
	return runMenuExitAction(manager, finalModel)
}

func runMenuProgram(ctx context.Context, menu tea.Model) (tea.Model, error) {
	// Bubble Tea installs its own SIGINT/SIGTERM handler by default, which
	// races with the signal.NotifyContext-driven ctx above: on a real
	// SIGINT it can independently return ErrProgramKilled wrapping
	// ErrInterrupted, an error that does not wrap ctx.Err() and so would be
	// (mis)treated as a genuine popup failure rather than graceful signal
	// shutdown. The runtime context is the single source of truth for
	// signal-driven cancellation (see the "Graceful signal shutdown"
	// architecture note), so disable Bubble Tea's competing handler.
	return tea.NewProgram(menu, tea.WithAltScreen(), tea.WithContext(ctx), tea.WithoutSignalHandler()).Run()
}

// isCancellationInducedPopupExit reports whether err is the kind of error
// Bubble Tea's Program.Run produces solely because the context passed via
// tea.WithContext was canceled, as opposed to a genuine, unrelated Bubble
// Tea or terminal error that happened to occur while a cancellation was
// also pending. Bubble Tea wraps every "killed" exit in ErrProgramKilled,
// including exits caused by a real underlying error (tea.go wraps it as
// fmt.Errorf("%w: %w", ErrProgramKilled, err) regardless of context state),
// so ErrProgramKilled alone cannot distinguish the two cases. What does
// distinguish them is whether the error chain also contains the exact
// context error the program was canceled with: a pure cancellation wraps
// ctx.Err() itself, while a real error wraps some other, unrelated error.
// Only the former should be swallowed as part of graceful signal shutdown;
// per the architecture, signal-driven cleanup must never replace another
// operation's real error.
//
// runMenuProgram disables Bubble Tea's own competing SIGINT/SIGTERM handler
// via tea.WithoutSignalHandler(), so in production this should be the only
// path that ever needs to distinguish these cases. As defense in depth,
// this also recognizes tea.ErrInterrupted: if Bubble Tea's own signal
// handler nonetheless fires (for example if it is ever re-enabled, or run
// through a different Program constructor), a real SIGINT reaching that
// handler while our own runtime context is also canceled is definitionally
// the same signal-driven shutdown, just detected through Bubble Tea's own
// mechanism rather than ctx -- so it must not be reported as an
// operational error either.
func isCancellationInducedPopupExit(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}
	ctxErr := ctx.Err()
	if ctxErr == nil {
		return false
	}
	if errors.Is(err, ctxErr) {
		return true
	}
	if errors.Is(err, tea.ErrInterrupted) {
		return true
	}
	return false
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
	cwd, _ := os.Getwd()
	return sessionStartDir(cwd)
}

func sessionStartDir(cwd string) string {
	cwd = strings.TrimSpace(cwd)
	if cwd != "" {
		if abs, err := filepath.Abs(cwd); err == nil {
			if info, err := os.Stat(abs); err == nil && info.IsDir() {
				return abs
			}
		}
	}
	for _, path := range []string{userHomeDir(), os.TempDir(), string(os.PathSeparator)} {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return filepath.Clean(path)
		}
	}
	return runtmux.NormalizeCWD("")
}

func userHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
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
