package ui

import "testing"

func TestSetSessionLabelPreservesCasing(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)
	m.sessions = []session{{Name: "small--session"}}
	m.setSessionLabel("small--session", "My Session")

	if got, want := m.sessionLabels["small--session"], "My Session"; got != want {
		t.Fatalf("stored label = %q, want %q", got, want)
	}
	if got, want := m.sessionLabel("small--session"), "My Session"; got != want {
		t.Fatalf("sessionLabel = %q, want %q", got, want)
	}
}

func TestSetSessionLabelTrimsWhitespaceOnly(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)
	m.sessions = []session{{Name: "small--session"}}
	m.setSessionLabel("small--session", "  Mixed CASE Label  ")

	if got, want := m.sessionLabels["small--session"], "Mixed CASE Label"; got != want {
		t.Fatalf("stored label = %q, want %q", got, want)
	}
}

func TestHasSessionLabelIsExactCaseSensitiveWithinProject(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)
	m.projects = []string{"small"}
	m.sessions = []session{{Name: "small--foo"}}
	m.sessionProjects = map[string]string{"small--foo": "small"}
	m.sessionLabels = map[string]string{"small--foo": "Foo"}

	if m.hasSessionLabel("small", "foo", "") {
		t.Fatal("hasSessionLabel treated \"foo\" as colliding with \"Foo\"")
	}
	if !m.hasSessionLabel("small", "Foo", "") {
		t.Fatal("hasSessionLabel did not reject exact-value duplicate \"Foo\"")
	}
}

func TestHasVolatileSessionLabelIsExactCaseSensitiveWithinInstance(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)
	m.instanceID = "one"
	m.sessions = []session{{Name: "tflow-v-one-a", Temporary: true, Instance: "one"}}
	m.sessionLabels = map[string]string{"tflow-v-one-a": "Notes"}

	if m.hasVolatileSessionLabel("notes", "") {
		t.Fatal("hasVolatileSessionLabel treated \"notes\" as colliding with \"Notes\"")
	}
	if !m.hasVolatileSessionLabel("Notes", "") {
		t.Fatal("hasVolatileSessionLabel did not reject exact-value duplicate \"Notes\"")
	}
}
