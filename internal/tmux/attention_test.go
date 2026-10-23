package tmux

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

func contentTestManager(panes map[string]string) Manager {
	return Manager{Run: func(args ...string) (string, error) {
		switch args[0] {
		case "list-panes":
			var ids []string
			for id := range panes {
				ids = append(ids, id)
			}
			return strings.Join(ids, "\n"), nil
		case "capture-pane":
			return panes[args[len(args)-1]], nil
		}
		return "", nil
	}}
}

func TestContentFingerprintComparesNormalizedTextAcrossAllPanes(t *testing.T) {
	panes := map[string]string{"%1": "prompt> \n\n", "%2": "old history\nresult\n"}
	m := contentTestManager(panes)
	before, err := m.sessionContentFingerprint("session")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, pane, text string
		changed          bool
	}{
		{"identical redraw", "%1", "prompt> \n\n", false},
		{"trailing padding", "%1", "prompt>   \n   \n\n", false},
		{"new foreground text", "%1", "prompt> done\n", true},
		{"background pane", "%2", "old history\nnew result\n", true},
		{"scrollback only", "%2", "new history\nresult\n", true},
		{"indentation", "%1", " prompt>\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := panes[tc.pane]
			panes[tc.pane] = tc.text
			defer func() { panes[tc.pane] = original }()
			after, err := m.sessionContentFingerprint("session")
			if err != nil {
				t.Fatal(err)
			}
			if (before != after) != tc.changed {
				t.Fatalf("changed=%v, want %v", before != after, tc.changed)
			}
		})
	}
}

func TestContentFingerprintEmptyAndStableOrder(t *testing.T) {
	panes := map[string]string{"%2": " \n\n", "%1": "\n"}
	m := contentTestManager(panes)
	got, err := m.sessionContentFingerprint("s")
	if err != nil || got != emptyContentFingerprint() {
		t.Fatalf("empty fingerprint=%q, err=%v", got, err)
	}
	panes["%2"] = "second"
	panes["%1"] = "first"
	first, _ := m.sessionContentFingerprint("s")
	for range 10 {
		got, err := m.sessionContentFingerprint("s")
		if err != nil || got != first {
			t.Fatalf("unstable pane order: %q %v", got, err)
		}
	}
}

func TestContentFingerprintIgnoresPaneIDReplacement(t *testing.T) {
	panes := map[string]string{"%1": "one\n", "%9": "two\n"}
	m := contentTestManager(panes)
	before, err := m.sessionContentFingerprint("s")
	if err != nil {
		t.Fatal(err)
	}
	delete(panes, "%1")
	panes["%20"] = "one\n" // The replacement sorts on the other side of %9.
	after, err := m.sessionContentFingerprint("s")
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("pane recreation changed fingerprint: %q != %q", before, after)
	}
	panes["%20"] = "new text\n"
	changed, err := m.sessionContentFingerprint("s")
	if err != nil {
		t.Fatal(err)
	}
	if changed == before {
		t.Fatal("changed pane text was ignored")
	}
}

func TestBaselineConditionAcceptsLongerRandText(t *testing.T) {
	baseline := strings.Repeat("A", 52) + ":" + emptyContentFingerprint()
	if _, err := baselineCondition(baseline); err != nil {
		t.Fatal(err)
	}
	tooLong := strings.Repeat("A", 257) + ":" + emptyContentFingerprint()
	if _, err := baselineCondition(tooLong); err == nil {
		t.Fatal("accepted oversized generation")
	}
}

func TestContentCaptureRequestsEveryWindowAndHistoryWithoutAttributes(t *testing.T) {
	var calls [][]string
	m := Manager{Run: func(args ...string) (string, error) {
		calls = append(calls, slices.Clone(args))
		if args[0] == "list-panes" {
			return "%3\n%1\n", nil
		}
		return "text", nil
	}}
	if _, err := m.sessionContentFingerprint("s"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(calls[0], []string{"list-panes", "-s", "-t", "s", "-F", "#{pane_id}"}) {
		t.Fatal(calls)
	}
	for i, pane := range []string{"%1", "%3"} {
		if !slices.Equal(calls[i+1], []string{"capture-pane", "-p", "-J", "-S", "-", "-E", "-", "-t", pane}) {
			t.Fatal(calls)
		}
	}
}

func TestContentCaptureFailureNeverWritesPartialBaseline(t *testing.T) {
	for _, failure := range []string{"missing pane", "new pane", "capture error"} {
		t.Run(failure, func(t *testing.T) {
			lists, writes := 0, 0
			m := Manager{Run: func(args ...string) (string, error) {
				switch args[0] {
				case "list-panes":
					lists++
					if failure == "new pane" && lists == 2 {
						return "%1\n%2\n%3", nil
					}
					return "%1\n%2", nil
				case "capture-pane":
					if args[len(args)-1] == "%2" {
						if failure == "missing pane" {
							return "", errors.New("can't find pane: %2")
						}
						if failure == "capture error" {
							return "", errors.New("capture failed")
						}
					}
					return "text", nil
				case "set-option":
					writes++
				}
				return "", nil
			}}
			err := m.MarkSessionVisited("s")
			if (err != nil) != (failure == "capture error") {
				t.Fatalf("error=%v", err)
			}
			if writes != 0 {
				t.Fatalf("published partial baseline: %d writes", writes)
			}
		})
	}
}

func TestRefreshAttentionIgnoresUnchangedContentAndLatchesNewText(t *testing.T) {
	panes := map[string]string{"%1": "idle Codex\n"}
	baseManager := contentTestManager(panes)
	fingerprint, _ := baseManager.sessionContentFingerprint("s")
	baseline := newContentBaseline(fingerprint)
	for _, tc := range []struct {
		name, text         string
		latched, wantWrite bool
	}{
		{"idle redraw", "idle Codex\n", false, false},
		{"changed text in same clock tick", "new output\n", false, true},
		{"already latched", "idle Codex\n", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			panes["%1"] = tc.text
			var write []string
			m := Manager{Run: func(args ...string) (string, error) {
				if args[0] == "set-option" {
					write = slices.Clone(args)
					return "1\n", nil
				}
				return baseManager.Run(args...)
			}}
			got, err := m.RefreshSessionAttention(Session{Name: "s", ContentBaseline: baseline, Attention: tc.latched})
			if err != nil || got != (tc.latched || tc.wantWrite) {
				t.Fatalf("attention=%v err=%v", got, err)
			}
			if (len(write) > 0) != tc.wantWrite {
				t.Fatalf("write=%v", write)
			}
			if tc.wantWrite && (!strings.Contains(write[5], baseline) || !strings.Contains(write[5], "#{==:#{session_attached},0}")) {
				t.Fatalf("unguarded write: %v", write)
			}
		})
	}
}

func TestVisitWritesAreGuardedByGenerationAndAttachment(t *testing.T) {
	for _, attached := range []bool{false, true} {
		t.Run(fmt.Sprint(attached), func(t *testing.T) {
			var call []string
			m := Manager{Run: func(args ...string) (string, error) { call = slices.Clone(args); return "", nil }}
			baseline := newContentBaseline(emptyContentFingerprint())
			if err := m.writeContentBaseline("s", baseline, emptyContentFingerprint(), attached); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(call[5], baseline) {
				t.Fatalf("missing generation guard: %v", call)
			}
			if strings.Contains(call[5], "#{>:#{session_attached},0}") != attached {
				t.Fatalf("incorrect attachment guard: %v", call)
			}
		})
	}
}

func TestBaselineConditionRejectsFormatInjection(t *testing.T) {
	for _, value := range []string{"#{session_name}", "a,1,0", newContentBaseline("not a hash")} {
		if _, err := baselineCondition(value); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
}
