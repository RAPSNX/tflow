package tmux

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
)

var errPaneSetChanged = errors.New("pane set changed during content capture")

func emptyContentFingerprint() string {
	return fmt.Sprintf("%x", sha256.Sum256(nil))
}

func newContentBaseline(fingerprint string) string {
	return rand.Text() + ":" + fingerprint
}

// baselineCondition accepts only our own format-safe tokens before embedding
// them in a tmux format. Empty denotes a session not yet initialized.
func baselineCondition(baseline string) (string, error) {
	if baseline != "" {
		generation, fingerprint, ok := strings.Cut(baseline, ":")
		decoded, err := hex.DecodeString(fingerprint)
		if !ok || (len(generation) == 0 || len(generation) > 256) || err != nil || len(decoded) != sha256.Size ||
			strings.IndexFunc(generation, func(r rune) bool {
				return !(r >= 'A' && r <= 'Z' || r >= '2' && r <= '7')
			}) >= 0 {
			return "", fmt.Errorf("invalid attention content baseline")
		}
	}
	return "#{==:#{" + contentMarker + "}," + baseline + "}", nil
}

// sessionContentFingerprint captures the application's screen (not copy-mode
// UI), including retained history. Text alone counts: tmux omits attributes,
// and -J joins soft wraps. Blank padding and trailing spaces are ignored.
func (m Manager) sessionContentFingerprint(name string) (string, error) {
	panes, err := m.contentPanes(name)
	if err != nil {
		return "", err
	}
	var contents []string
	for _, pane := range panes {
		out, err := m.runner()("capture-pane", "-p", "-J", "-S", "-", "-E", "-", "-t", pane)
		if err != nil {
			if strings.Contains(err.Error(), "can't find pane") {
				return "", errPaneSetChanged
			}
			return "", err
		}
		text := normalizePaneText(out)
		if text != "" {
			contents = append(contents, text)
		}
	}
	// Never publish a partial snapshot when a pane was added or removed.
	after, err := m.contentPanes(name)
	if err != nil {
		return "", err
	}
	if !slices.Equal(panes, after) {
		return "", errPaneSetChanged
	}
	// Pane IDs only identify what to capture. A recreated pane with the same
	// text has the same fingerprint, even if its new ID sorts elsewhere.
	slices.Sort(contents)
	h := sha256.New()
	for _, text := range contents {
		fmt.Fprintf(h, "%d:%s", len(text), text)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (m Manager) contentPanes(name string) ([]string, error) {
	out, err := m.runner()("list-panes", "-s", "-t", name, "-F", "#{pane_id}")
	if err != nil {
		return nil, err
	}
	panes := strings.Fields(out)
	if len(panes) == 0 {
		return nil, errPaneSetChanged
	}
	slices.Sort(panes)
	return slices.Compact(panes), nil
}

func normalizePaneText(text string) string {
	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t\r")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

// writeContentBaseline uses a generation comparison to discard an old capture
// after another visit. attachedOnly additionally protects a status job whose
// session detached while it was capturing. Both writes are evaluated inside
// tmux, rather than relying on a Go-side attachment check.
func (m Manager) writeContentBaseline(name, expected, fingerprint string, attachedOnly bool) error {
	condition, err := baselineCondition(expected)
	if err != nil {
		return err
	}
	if attachedOnly {
		condition = "#{&&:" + condition + ",#{>:#{session_attached},0}}"
	}
	baseline := newContentBaseline(fingerprint)
	_, err = m.runBatch(
		[]string{"set-option", "-F", "-t", name, contentMarker,
			"#{?" + condition + "," + baseline + ",#{" + contentMarker + "}}"},
		[]string{"set-option", "-F", "-t", name, attentionMarker,
			"#{?#{==:#{" + contentMarker + "}," + baseline + "},0,#{" + attentionMarker + "}}"},
	)
	return err
}

// InitializeSessionAttention only initializes a missing baseline, preserving
// unread attention when another tflow instance starts on the same server.
func (m Manager) InitializeSessionAttention(name string) error {
	return m.captureVisit(name, "", false)
}

func (m Manager) captureVisit(name, expected string, attachedOnly bool) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("session name is empty")
	}
	fingerprint, err := m.sessionContentFingerprint(name)
	if errors.Is(err, errPaneSetChanged) {
		return nil // Retry on the next status tick, without publishing partial text.
	}
	if err != nil {
		return err
	}
	return m.writeContentBaseline(name, expected, fingerprint, attachedOnly)
}

func (m Manager) MarkSessionVisited(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("session name is empty")
	}
	expected, err := m.runner()("display-message", "-p", "-t", name, "#{"+contentMarker+"}")
	if err != nil {
		return err
	}
	return m.captureVisit(name, strings.TrimSpace(expected), false)
}

// RefreshSessionAttention compares text directly, including within one tmux
// clock tick. A marker stays latched until a visit. Already-flagged unattached
// sessions do not need their scrollback captured again.
func (m Manager) RefreshSessionAttention(s Session) (bool, error) {
	if s.ContentBaseline == "" || s.Attached {
		err := m.captureVisit(s.Name, s.ContentBaseline, s.Attached)
		if err != nil {
			return s.Attention, err
		}
		out, err := m.runner()("display-message", "-p", "-t", s.Name, "#{"+attentionMarker+"}")
		return strings.TrimSpace(out) == "1", err
	}
	if s.Attention {
		return true, nil
	}
	condition, err := baselineCondition(s.ContentBaseline)
	if err != nil {
		return false, err
	}
	fingerprint, err := m.sessionContentFingerprint(s.Name)
	if errors.Is(err, errPaneSetChanged) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_, seen, _ := strings.Cut(s.ContentBaseline, ":")
	if fingerprint == seen {
		return false, nil
	}
	condition = "#{&&:" + condition + ",#{==:#{session_attached},0}}"
	out, err := m.runBatch(
		[]string{"set-option", "-F", "-t", s.Name, attentionMarker,
			"#{?" + condition + ",1,#{" + attentionMarker + "}}"},
		[]string{"display-message", "-p", "-t", s.Name, "#{" + attentionMarker + "}"},
	)
	return strings.TrimSpace(out) == "1", err
}
