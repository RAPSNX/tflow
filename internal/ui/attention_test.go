package ui

import "testing"

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
				{Name: "busy-unvisited", Activity: true},
				{Name: "busy-attached", Activity: true, Attached: true},
				{Name: "already-flagged", Activity: true, Attention: true},
				{Name: "idle", Activity: false},
			}, nil
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
	t.Setenv(menuInstanceEnv, "inst-1")
	if err := attentionScanWithManager(fake); err != nil {
		t.Fatalf("attentionScanWithManager: %v", err)
	}
	if pushedName != "s1" || pushedContent == "" {
		t.Fatalf("pushedName=%q pushedContent=%q, want a refreshed top bar for s1", pushedName, pushedContent)
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
