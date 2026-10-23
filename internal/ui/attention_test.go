package ui

import (
	"strings"
	"testing"
)

func TestSessionVisitedStampsBothTheEnteredAndOutgoingSession(t *testing.T) {
	var visited []string
	fake := fakeTmuxController{
		markSessionVisited: func(name string) error {
			visited = append(visited, name)
			return nil
		},
	}

	t.Setenv(menuCurrentEnv, "destination")
	t.Setenv(menuLastVisitedEnv, "outgoing")
	if err := sessionVisitedWithManager(fake); err != nil {
		t.Fatalf("sessionVisitedWithManager: %v", err)
	}
	if want := []string{"destination", "outgoing"}; !slicesEqual(visited, want) {
		t.Fatalf("visited = %#v, want %#v", visited, want)
	}
}

// TestSessionVisitedDoesNotDoubleStampWhenReenteringTheSameSession guards
// against a spurious duplicate MarkSessionVisited call when
// client_last_session reports the same session tmux just switched into
// (e.g. re-selecting the already-current session).
func TestSessionVisitedDoesNotDoubleStampWhenReenteringTheSameSession(t *testing.T) {
	var visited []string
	fake := fakeTmuxController{
		markSessionVisited: func(name string) error {
			visited = append(visited, name)
			return nil
		},
	}

	t.Setenv(menuCurrentEnv, "s1")
	t.Setenv(menuLastVisitedEnv, "s1")
	if err := sessionVisitedWithManager(fake); err != nil {
		t.Fatalf("sessionVisitedWithManager: %v", err)
	}
	if want := []string{"s1"}; !slicesEqual(visited, want) {
		t.Fatalf("visited = %#v, want %#v", visited, want)
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestAttentionScanRefreshesCurrentSessionTopBar(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	var pushedName, pushedContent string
	fake := fakeTmuxController{
		listSessions: func() ([]session, error) {
			return []session{{Name: "s1", Temporary: true, Instance: "inst-1", Label: "code"}}, nil
		},
		refreshSessionAttention: func(s session) (bool, error) { return s.Attention, nil },
		setSessionTopBar: func(name, content string) error {
			pushedName, pushedContent = name, content
			return nil
		},
	}

	t.Setenv(menuCurrentEnv, "s1")
	if err := attentionScanWithManager(fake); err != nil {
		t.Fatalf("attentionScanWithManager: %v", err)
	}
	if pushedName != "s1" || pushedContent == "" {
		t.Fatalf("pushedName=%q pushedContent=%q, want a refreshed top bar for s1", pushedName, pushedContent)
	}
}

// TestAttentionScanResolvesInstanceFromCurrentSessionNotEnv guards against a
// regression where the instance ID was read from TFLOW_INSTANCE_ID, an env
// var the status #() job never exports. An unresolved (empty) instance ID
// makes computeTargetTopBar's volatile branch treat every instance's
// sessions as its own, bleeding a different instance's volatile sessions
// into this one's top bar on every two-second scan.
func TestAttentionScanResolvesInstanceFromCurrentSessionNotEnv(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	var pushedContent string
	fake := fakeTmuxController{
		listSessions: func() ([]session, error) {
			return []session{
				{Name: "s1", Temporary: true, Instance: "inst-1", Label: "mine"},
				{Name: "s2", Temporary: true, Instance: "inst-2", Label: "other"},
			}, nil
		},
		refreshSessionAttention: func(s session) (bool, error) { return s.Attention, nil },
		setSessionTopBar: func(name, content string) error {
			pushedContent = content
			return nil
		},
	}

	// Deliberately leave menuInstanceEnv unset: the real #() job never sets
	// it either, so the fix must resolve the instance from the current
	// session's own tmux marker instead.
	t.Setenv(menuCurrentEnv, "s1")
	if err := attentionScanWithManager(fake); err != nil {
		t.Fatalf("attentionScanWithManager: %v", err)
	}
	if !strings.Contains(pushedContent, "mine") {
		t.Fatalf("pushedContent = %q, want the current instance's own session label", pushedContent)
	}
	if strings.Contains(pushedContent, "other") {
		t.Fatalf("pushedContent = %q, leaked a different instance's volatile session", pushedContent)
	}
}

func TestAttentionScanUsesLiveAttachmentNotCurrentEnvironment(t *testing.T) {
	t.Setenv(menuCurrentEnv, "stale-current")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	var refreshed []string
	fake := fakeTmuxController{
		listSessions: func() ([]session, error) {
			return []session{{Name: "stale-current"}, {Name: "viewed-by-other-client", Attached: true}}, nil
		},
		refreshSessionAttention: func(s session) (bool, error) {
			refreshed = append(refreshed, s.Name)
			if s.Name == "stale-current" && s.Attached {
				t.Fatal("stale environment counted as viewing")
			}
			return !s.Attached, nil
		},
	}
	if err := attentionScanWithManager(fake); err != nil {
		t.Fatal(err)
	}
	if len(refreshed) != 2 {
		t.Fatalf("refreshed=%v", refreshed)
	}
}

func TestStartupInitializesOnlyMissingBaselinesBeforeEnablingScan(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	var initialized []string
	fake := fakeTmuxController{
		listSessions: func() ([]session, error) {
			return []session{
				{Name: "preexisting"},
				{Name: "already-tracked", ContentBaseline: "established", Attention: true},
			}, nil
		},
		initializeSessionAttention: func(name string) error { initialized = append(initialized, name); return nil },
		ensureControlMode: func(string) error {
			if !slicesEqual(initialized, []string{"preexisting"}) {
				t.Fatalf("initialized=%v", initialized)
			}
			return nil
		},
	}
	if _, err := prepareStartup(fake, "/tmp/tflow", "/tmp", "instance"); err != nil {
		t.Fatal(err)
	}
}
