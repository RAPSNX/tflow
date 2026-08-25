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
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/rapsnx/tflow/internal/diag"
	"github.com/rapsnx/tflow/internal/store"
	runtmux "github.com/rapsnx/tflow/internal/tmux"
)

// attachTerminationWaitDelay bounds how long a canceled attach client gets
// to exit on its own after being asked to terminate gracefully before it is
// force-killed. It is a variable so tests can shrink it instead of waiting
// out a production-sized grace period.
var attachTerminationWaitDelay = 3 * time.Second

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
	sessionName, err := prepareStartup(manager, binaryPath, cwd, instanceID)
	if err != nil {
		return err
	}
	// cleanupFailureContext names the branch that was returning (with its own
	// error already decided) when the single deferred cleanup below ran. When
	// that branch already has a real error to report, the cleanup error can't
	// also become the return value, so it is reported as a diagnostic instead
	// with a message identifying which branch swallowed it.
	cleanupFailureContext := ""
	defer func() {
		cleanupErr := manager.CleanupVolatileSessions(instanceID)
		if cleanupErr == nil {
			return
		}
		if result == nil {
			result = cleanupErr
			return
		}
		// Preserve the already-decided error; volatile cleanup is deliberately best effort.
		diag.Warnf("clean up volatile sessions for instance %q after %s: %v", instanceID, cleanupFailureContext, cleanupErr)
	}()

	// Signal interception begins here, at the attach boundary, not before
	// prepareStartup.
	ctx, stop := newAttachContext()
	defer stop()

	cmd, err := manager.AttachCommand(ctx, sessionName)
	if err != nil {
		cleanupFailureContext = "attach failure"
		return err
	}
	// Ask the attached client to terminate gracefully on cancellation instead
	// of the exec package's default of an immediate Process.Kill(), so it
	// gets a chance to restore terminal state (raw mode, alternate screen)
	// before exiting. WaitDelay bounds how long that grace period lasts; if
	// the client hasn't exited by then, Wait force-kills it, matching the
	// architecture's "bounded wait before forcefully terminating it."
	//
	// canceled records whether the exec package actually invoked Cancel for
	// this process and the SIGTERM it sent was actually delivered to a
	// still-running process, independent of how the process then exited. A
	// client that traps SIGTERM and exits cleanly-but-nonzero would
	// otherwise be indistinguishable from a genuine operational failure:
	// isCancellationInducedAttachFailure alone only recognizes the
	// signal-terminated shape a bare Process.Kill() always produced; a
	// graceful SIGTERM handler can legitimately produce any exit shape.
	// canceled is authoritative for that case because Cancel only runs when
	// this cmd's own context (ctx, passed to AttachCommand) is done, so it
	// can't be true for a failure the test suite deliberately ties to an
	// unrelated, never-canceled context.
	//
	// Cancel can still fire after the process has already exited on its own
	// for an unrelated, genuine reason (e.g. the attach client failed at
	// roughly the same moment a signal arrived) -- ctx cancellation and the
	// process's own exit are observed through independent paths, so there is
	// no ordering guarantee between them. Signal then returns
	// os.ErrProcessDone rather than delivering anything. Only set canceled
	// when the signal actually reached a live process; otherwise the
	// process's own exit error must still be reported, not swallowed as if
	// our signal had caused it.
	canceled := false
	cmd.Cancel = func() error {
		err := cmd.Process.Signal(syscall.SIGTERM)
		if err == nil {
			canceled = true
		}
		return err
	}
	cmd.WaitDelay = attachTerminationWaitDelay
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil && (canceled || isCancellationInducedAttachFailure(err)) {
			return nil
		}
		cleanupFailureContext = "client error"
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
	repairPersistentSessionMarkers(manager, path, existing)
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

// repairPersistentSessionMarkers restores the project and label markers
// persisted state expects for every persistent session that survived
// reconciliation, and clears any stale volatile ownership markers an
// interrupted creation, promotion, or move may have left behind. It is
// strictly limited to sessions represented in state: it never touches a
// session state does not know about. Repair failures are best effort and
// left for the next startup reconciliation, matching the architecture's
// error-handling rules for non-critical inconsistencies.
//
// The state read and the tmux marker writes it drives run under the same
// state lock a concurrent mutation (rename, move, promotion) would hold.
// Without that, a state snapshot read here could go stale between the read
// and the writes below, and this repair pass could overwrite a concurrent
// instance's fresher markers with the older values it just read -- the
// exact inconsistency this repair exists to fix, reintroduced by reading
// state without serializing against writers. ReconcileAppState already
// calls a real tmux command (its snapshotSessions callback) while holding
// this same lock, so holding it across the tmux writes here follows the
// same established pattern.
func repairPersistentSessionMarkers(manager tmuxController, path string, existing []session) {
	unlock, err := lockAppState(path)
	if err != nil {
		diag.Warnf("lock state %q for startup marker repair: %v", path, err)
		return
	}
	defer func() {
		if unlockErr := unlock(); unlockErr != nil {
			diag.Warnf("release state lock %q after startup marker repair: %v", path, unlockErr)
		}
	}()
	state, err := loadAppState(path)
	if err != nil {
		diag.Warnf("load state %q for startup marker repair: %v", path, err)
		return
	}
	existingNames := make(map[string]struct{}, len(existing))
	for _, s := range existing {
		existingNames[s.Name] = struct{}{}
	}
	for _, project := range state.Projects {
		for _, persistent := range project.Sessions {
			if _, ok := existingNames[persistent.ID]; !ok {
				continue
			}
			if err := manager.SetSessionProject(persistent.ID, project.Name); err != nil {
				diag.Warnf("repair project marker for session %q: %v", persistent.ID, err)
			}
			if err := manager.SetSessionLabel(persistent.ID, persistent.Label); err != nil {
				diag.Warnf("repair label marker for session %q: %v", persistent.ID, err)
			}
			if err := manager.SetSessionTemporary(persistent.ID, false, ""); err != nil {
				diag.Warnf("clear stale volatile markers for session %q: %v", persistent.ID, err)
			}
		}
	}
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
		target := strings.TrimSpace(menu.exitSessionName)
		if target == "" {
			return nil
		}
		// Detect whether the outgoing session's panes have all exited before
		// executing the switch, per the architecture. A check failure is
		// reported as a diagnostic and treated as "not dead" -- it must never
		// block the switch itself, which is the primary operation here.
		outgoing := strings.TrimSpace(menu.currentSession)
		deletingAfterSwitch := len(menu.exitDeleteSessions) > 0
		removable := !deletingAfterSwitch && outgoing != "" && outgoing != target && outgoingSessionPanesAllDead(manager, outgoing)
		if err := manager.SwitchClient(target); err != nil {
			return err
		}
		if deletingAfterSwitch {
			deletePersistentSessionsAfterSwitch(manager, menu)
			return nil
		}
		if removable {
			removeDeadOutgoingSession(manager, menu, outgoing)
		}
		return nil
	case menuExitQuit:
		return manager.QuitAll()
	default:
		return nil
	}
}

func outgoingSessionPanesAllDead(manager tmuxController, outgoing string) bool {
	allDead, err := manager.SessionPanesAllDead(outgoing)
	if err != nil {
		diag.Warnf("check dead panes for outgoing session %q: %v", outgoing, err)
		return false
	}
	return allDead
}

// removeDeadOutgoingSession removes an outgoing session already confirmed,
// before the switch, to have every pane exited (direct session selection
// and project selection both resolve to the same exit action, so this
// covers both). Every step here is best effort: a failure is reported as a
// diagnostic and never replaces the fact that the switch itself already
// succeeded, matching the architecture's "keep the selected target active
// and report a diagnostic when post-switch cleanup ... fails."
func removeDeadOutgoingSession(manager tmuxController, menu model, outgoing string) {
	if err := manager.KillSession(outgoing); err != nil {
		diag.Warnf("kill dead outgoing session %q: %v", outgoing, err)
		return
	}
	info, found := menu.findSession(outgoing)
	if !found || info.Temporary {
		// Volatile sessions carry no persistent metadata to remove; an
		// unrecognized session is left for the next startup reconciliation.
		return
	}
	path := appStatePath()
	if _, err := mutateAppState(path, func(state appState) (appState, error) {
		return store.RemoveSession(state, outgoing), nil
	}); err != nil {
		diag.Warnf("remove persisted metadata for dead outgoing session %q: %v", outgoing, err)
	}
}

// deletePersistentSessionsAfterSwitch removes explicit deletion targets only
// after the fallback client switch succeeded. This keeps the client attached
// throughout active final-session and active-project deletion. Failures are
// diagnostics because the fallback switch is already the completed primary
// operation.
func deletePersistentSessionsAfterSwitch(manager tmuxController, menu model) {
	for _, name := range menu.exitDeleteSessions {
		if err := ignoreMissingSession(manager.KillSession(name)); err != nil {
			diag.Warnf("delete session %q after fallback switch: %v", name, err)
			return
		}
	}
	path := menu.statePath
	if strings.TrimSpace(path) == "" {
		path = appStatePath()
	}
	if _, err := mutateAppState(path, func(state appState) (appState, error) {
		for _, name := range menu.exitDeleteSessions {
			state = store.RemoveSession(state, name)
		}
		return state, nil
	}); err != nil {
		diag.Warnf("remove persistent metadata after fallback switch: %v", err)
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
