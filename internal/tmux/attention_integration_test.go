package tmux

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Run with TFLOW_TMUX_VERIFY_ROOT=$ROOT after scripts/tmux-verify.sh setup.
// The helper owns socket setup/teardown; this test only owns its test session.
func TestAttentionOnIsolatedTmux(t *testing.T) {
	root := os.Getenv("TFLOW_TMUX_VERIFY_ROOT")
	if root == "" {
		t.Skip("requires scripts/tmux-verify.sh setup")
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil || resolved != root || !strings.HasPrefix(filepath.Base(root), "tmux-verify.") ||
		os.Getenv("TMUX_TMPDIR") != filepath.Join(root, "tmux") {
		t.Fatal("verification root must be the helper-created isolated environment")
	}
	t.Setenv("TMUX", "")
	t.Setenv("SHELL", "/bin/sh")
	t.Setenv("TERM", "xterm-256color")
	m := Manager{}
	name := "attention-integration"
	if _, err := m.CreateSession(name, root, "stty -echo; PS1='' exec /bin/sh"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.KillSession(name) })
	run := func(args ...string) string {
		t.Helper()
		out, err := Run(args...)
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(out)
	}
	snapshot := func() Session {
		t.Helper()
		ss, err := m.ListSessions()
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range ss {
			if s.Name == name {
				return s
			}
		}
		t.Fatal("session disappeared")
		return Session{}
	}
	await := func(check func() bool) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if check() {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("timed out")
	}
	write := func(text string) {
		t.Helper()
		run("send-keys", "-t", name, "printf '%s\\n' "+ShellQuote(text), "Enter")
		await(func() bool { return strings.Contains(run("capture-pane", "-p", "-t", name), text) })
	}
	time.Sleep(100 * time.Millisecond) // Let stty disable terminal echo before sending commands.
	write("seen text")
	if marked, err := m.RefreshSessionAttention(snapshot()); err != nil || !marked {
		t.Fatalf("new session's initial unviewed text: marked=%v err=%v", marked, err)
	}
	if err := m.MarkSessionVisited(name); err != nil {
		t.Fatal(err)
	}
	t.Run("control output does not alert", func(t *testing.T) {
		run("send-keys", "-t", name, "printf '\\033[?25l\\033[?25h\\033[31m\\033[0m'", "Enter")
		time.Sleep(100 * time.Millisecond)
		marked, err := m.RefreshSessionAttention(snapshot())
		if err != nil || marked {
			t.Fatalf("marked=%v err=%v", marked, err)
		}
	})
	t.Run("fresh text latches and visit clears", func(t *testing.T) {
		write("unseen result")
		marked, err := m.RefreshSessionAttention(snapshot())
		if err != nil || !marked {
			t.Fatalf("marked=%v err=%v", marked, err)
		}
		if err := m.InitializeSessionAttention(name); err != nil {
			t.Fatal(err)
		}
		if !snapshot().Attention {
			t.Fatal("initialization erased established unread marker")
		}
		if err := m.MarkSessionVisited(name); err != nil {
			t.Fatal(err)
		}
		if snapshot().Attention {
			t.Fatal("visit did not clear marker")
		}
	})
	t.Run("concurrent visit rejects stale scan", func(t *testing.T) {
		write("concurrent result")
		injected := false
		racing := Manager{Run: func(args ...string) (string, error) {
			if args[0] == "set-option" && !injected {
				injected = true
				if err := m.MarkSessionVisited(name); err != nil {
					return "", err
				}
			}
			return Run(args...)
		}}
		marked, err := racing.RefreshSessionAttention(snapshot())
		if err != nil || marked || snapshot().Attention || !injected {
			t.Fatalf("marked=%v injected=%v err=%v", marked, injected, err)
		}
	})
	t.Run("detached client rejects attached scan", func(t *testing.T) {
		before := snapshot()
		write("after detachment")
		stale := before
		stale.Attached = true
		if _, err := m.RefreshSessionAttention(stale); err != nil {
			t.Fatal(err)
		}
		if snapshot().ContentBaseline != before.ContentBaseline {
			t.Fatal("stale attached scan advanced baseline")
		}
		if err := m.MarkSessionVisited(name); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("client attaching during capture prevents marker", func(t *testing.T) {
		write("before attach")
		var client *exec.Cmd
		injected := false
		racing := Manager{Run: func(args ...string) (string, error) {
			if args[0] == "set-option" && !injected {
				injected = true
				client = exec.Command("script", "-qec", "stty rows 24 cols 80; exec tmux -L tflow attach-session -t "+name, "/dev/null")
				reader, writer := io.Pipe()
				client.Stdin = reader
				client.Stdout = io.Discard
				client.Stderr = io.Discard
				if err := client.Start(); err != nil {
					return "", err
				}
				t.Cleanup(func() {
					_, _ = Run("detach-client", "-s", name)
					_ = writer.Close()
					_ = reader.Close()
					_ = client.Wait()
				})
				await(func() bool { attached, _ := m.SessionAttached(name); return attached })
			}
			return Run(args...)
		}}
		marked, err := racing.RefreshSessionAttention(snapshot())
		if err != nil || marked || snapshot().Attention || !injected {
			t.Fatalf("marked=%v injected=%v err=%v", marked, injected, err)
		}
	})
}
