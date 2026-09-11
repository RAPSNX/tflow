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
