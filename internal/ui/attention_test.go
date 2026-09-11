package ui

import (
	"strings"
	"testing"
)

func TestSessionActivityMarksOnlyUnvisitedSession(t *testing.T) {
	var marked map[string]bool
	fake := fakeTmuxController{
		sessionAttached: func(name string) (bool, error) {
			return name == "visited", nil
		},
		setSessionAttention: func(name string, attention bool) error {
			if marked == nil {
				marked = map[string]bool{}
			}
			marked[name] = attention
			return nil
		},
	}

	t.Setenv(menuCurrentEnv, "unvisited")
	if err := sessionActivityWithManager(fake); err != nil {
		t.Fatalf("sessionActivityWithManager: %v", err)
	}
	if !marked["unvisited"] {
		t.Fatalf("marked = %#v, want unvisited session marked", marked)
	}

	marked = nil
	t.Setenv(menuCurrentEnv, "visited")
	if err := sessionActivityWithManager(fake); err != nil {
		t.Fatalf("sessionActivityWithManager: %v", err)
	}
	if marked != nil {
		t.Fatalf("marked = %#v, want no marker set for an attached (visited) session", marked)
	}
}

func TestSessionActivityIgnoresMissingCurrentSession(t *testing.T) {
	called := false
	fake := fakeTmuxController{
		sessionAttached: func(name string) (bool, error) { return false, nil },
		setSessionAttention: func(name string, attention bool) error {
			called = true
			return nil
		},
	}
	t.Setenv(menuCurrentEnv, "")
	if err := sessionActivityWithManager(fake); err != nil {
		t.Fatalf("sessionActivityWithManager: %v", err)
	}
	if called {
		t.Fatal("expected no attention call without a current session")
	}
}

func TestSessionVisitedClearsAttentionUnconditionally(t *testing.T) {
	var name string
	var attention bool
	called := false
	fake := fakeTmuxController{
		setSessionAttention: func(n string, a bool) error {
			name, attention, called = n, a, true
			return nil
		},
	}
	t.Setenv(menuCurrentEnv, "s1")
	if err := sessionVisitedWithManager(fake); err != nil {
		t.Fatalf("sessionVisitedWithManager: %v", err)
	}
	if !called || name != "s1" || attention {
		t.Fatalf("name=%q attention=%v called=%v, want clearing s1", name, attention, called)
	}
}

func TestAttentionScanMarksUnvisitedSessionsWithFreshActivity(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	marked := map[string]bool{}
	fake := fakeTmuxController{
		listSessions: func() ([]session, error) {
			return []session{
				{Name: "busy-unvisited"},
				{Name: "busy-attached", Attached: true},
				{Name: "already-flagged", Attention: true},
				{Name: "idle"},
			}, nil
		},
		windowActivityBySession: func() (map[string]bool, error) {
			return map[string]bool{"busy-unvisited": true, "busy-attached": true, "already-flagged": true}, nil
		},
		setSessionAttention: func(name string, attention bool) error {
			marked[name] = attention
			return nil
		},
	}

	t.Setenv(menuCurrentEnv, "")
	if err := attentionScanWithManager(fake); err != nil {
		t.Fatalf("attentionScanWithManager: %v", err)
	}
	if want := map[string]bool{"busy-unvisited": true}; !mapsEqual(marked, want) {
		t.Fatalf("marked = %#v, want %#v", marked, want)
	}
}

func TestAttentionScanUsesEveryWindowNotJustTheActiveOne(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	marked := map[string]bool{}
	fake := fakeTmuxController{
		listSessions: func() ([]session, error) {
			return []session{{Name: "multi-window"}}, nil
		},
		// Simulates activity in a background (non-active) window: list-sessions'
		// own window_activity_flag sample would miss this, but the aggregated
		// per-session view from WindowActivityBySession must not.
		windowActivityBySession: func() (map[string]bool, error) {
			return map[string]bool{"multi-window": true}, nil
		},
		setSessionAttention: func(name string, attention bool) error {
			marked[name] = attention
			return nil
		},
	}

	t.Setenv(menuCurrentEnv, "")
	if err := attentionScanWithManager(fake); err != nil {
		t.Fatalf("attentionScanWithManager: %v", err)
	}
	if !marked["multi-window"] {
		t.Fatalf("marked = %#v, want multi-window session marked from its background window", marked)
	}
}

// TestAttentionScanRechecksAttachmentBeforeSettingTheMarker guards against a
// race where a client attaches to (or is visited on) a session between the
// ListSessions snapshot and the SetSessionAttention write: nothing later
// re-clears attention for an attached session, so writing from the stale
// snapshot would leave the marker incorrectly set on the currently viewed
// session until some unrelated future visit.
func TestAttentionScanRechecksAttachmentBeforeSettingTheMarker(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	marked := map[string]bool{}
	fake := fakeTmuxController{
		listSessions: func() ([]session, error) {
			// Snapshot says unattached, but a client attaches in the gap
			// before the write below runs.
			return []session{{Name: "just-attached", Attached: false}}, nil
		},
		windowActivityBySession: func() (map[string]bool, error) {
			return map[string]bool{"just-attached": true}, nil
		},
		sessionAttached: func(name string) (bool, error) {
			return name == "just-attached", nil
		},
		setSessionAttention: func(name string, attention bool) error {
			marked[name] = attention
			return nil
		},
	}

	t.Setenv(menuCurrentEnv, "")
	if err := attentionScanWithManager(fake); err != nil {
		t.Fatalf("attentionScanWithManager: %v", err)
	}
	if marked["just-attached"] {
		t.Fatalf("marked = %#v, want no marker set for a session attached at recheck time", marked)
	}
}

func TestAttentionScanRefreshesCurrentSessionTopBar(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	var pushedName, pushedContent string
	fake := fakeTmuxController{
		listSessions: func() ([]session, error) {
			return []session{{Name: "s1", Temporary: true, Instance: "inst-1", Label: "code"}}, nil
		},
		setSessionAttention: func(name string, attention bool) error { return nil },
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
		setSessionAttention: func(name string, attention bool) error { return nil },
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

func mapsEqual(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
