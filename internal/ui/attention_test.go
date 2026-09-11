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

func TestSessionVisitedClearsAttentionAndStampsWatermark(t *testing.T) {
	var clearedName string
	var attention bool
	var visitedName string
	fake := fakeTmuxController{
		setSessionAttention: func(n string, a bool) error {
			clearedName, attention = n, a
			return nil
		},
		markSessionVisited: func(n string) error {
			visitedName = n
			return nil
		},
	}
	t.Setenv(menuCurrentEnv, "s1")
	if err := sessionVisitedWithManager(fake); err != nil {
		t.Fatalf("sessionVisitedWithManager: %v", err)
	}
	if visitedName != "s1" {
		t.Fatalf("visitedName=%q, want MarkSessionVisited called for s1", visitedName)
	}
	if clearedName != "" || attention {
		t.Fatalf("clearedName=%q attention=%v, want SetSessionAttention not called directly -- MarkSessionVisited owns clearing", clearedName, attention)
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
		sessionActivityTimestamps: func() (map[string]int64, error) {
			return map[string]int64{"busy-unvisited": 100, "busy-attached": 100, "already-flagged": 100}, nil
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
		// per-session view from SessionActivityTimestamps must not.
		sessionActivityTimestamps: func() (map[string]int64, error) {
			return map[string]int64{"multi-window": 100}, nil
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

// TestAttentionScanIgnoresActivityThatPredatesTheLastVisit guards against a
// regression where a background (non-active) window's tmux activity flag --
// only cleared by individually selecting that window, not by visiting the
// session -- stayed set from before the last visit and was misread as fresh
// output on every subsequent scan, permanently re-flagging the session.
func TestAttentionScanIgnoresActivityThatPredatesTheLastVisit(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	marked := map[string]bool{}
	fake := fakeTmuxController{
		listSessions: func() ([]session, error) {
			return []session{{Name: "stale-background-window", VisitedAt: 500}}, nil
		},
		sessionActivityTimestamps: func() (map[string]int64, error) {
			return map[string]int64{"stale-background-window": 100}, nil
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
	if marked["stale-background-window"] {
		t.Fatalf("marked = %#v, want no marker set for activity that predates the last visit", marked)
	}
}

func TestAttentionScanMarksActivityNewerThanTheLastVisit(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	marked := map[string]bool{}
	fake := fakeTmuxController{
		listSessions: func() ([]session, error) {
			return []session{{Name: "s1", VisitedAt: 100}}, nil
		},
		sessionActivityTimestamps: func() (map[string]int64, error) {
			return map[string]int64{"s1": 200}, nil
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
	if !marked["s1"] {
		t.Fatalf("marked = %#v, want the marker set for activity newer than the last visit", marked)
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
		sessionActivityTimestamps: func() (map[string]int64, error) {
			return map[string]int64{"just-attached": 100}, nil
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

// TestAttentionScanRefreshesCurrentSessionWatermarkEveryTick guards against
// a regression where only client-session-changed (on entry) stamped a
// session's watermark: output produced while a session was actively being
// viewed would still look unseen the instant the client switched away,
// because the watermark was frozen at entry time while window_activity kept
// advancing throughout the visit.
func TestAttentionScanRefreshesCurrentSessionWatermarkEveryTick(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	var visited []string
	fake := fakeTmuxController{
		listSessions: func() ([]session, error) {
			return []session{{Name: "viewed", Attached: true}}, nil
		},
		markSessionVisited: func(name string) error {
			visited = append(visited, name)
			return nil
		},
	}

	t.Setenv(menuCurrentEnv, "viewed")
	if err := attentionScanWithManager(fake); err != nil {
		t.Fatalf("attentionScanWithManager: %v", err)
	}
	if len(visited) != 1 || visited[0] != "viewed" {
		t.Fatalf("visited = %#v, want the current session's watermark refreshed every scan", visited)
	}
}

// TestAttentionScanTreatsEqualWatermarkAsNotFresh guards the property that
// makes the watermark comparison unambiguous at one-second resolution:
// VisitedAt is stamped from the session's own peak window_activity at visit
// time (see MarkSessionVisited), not wall-clock time, so activity exactly
// equal to the watermark means window_activity hasn't advanced since the
// visit -- nothing new happened, regardless of what wall-clock second either
// value falls in.
func TestAttentionScanTreatsEqualWatermarkAsNotFresh(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	marked := map[string]bool{}
	fake := fakeTmuxController{
		listSessions: func() ([]session, error) {
			return []session{{Name: "s1", VisitedAt: 1000}}, nil
		},
		sessionActivityTimestamps: func() (map[string]int64, error) {
			return map[string]int64{"s1": 1000}, nil
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
	if marked["s1"] {
		t.Fatalf("marked = %#v, want activity equal to the watermark treated as not fresh", marked)
	}
}

// TestSessionVisitedStampsBothTheEnteredAndOutgoingSession guards against a
// regression where only the destination session's watermark was refreshed
// on a switch: output produced in the gap between AttentionScan's last tick
// and the moment of switching away would otherwise still look unseen once
// the outgoing session is detached, since nothing else would refresh its
// watermark until it was visited again.
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
