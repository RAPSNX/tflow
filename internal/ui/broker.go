package ui

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/creack/pty"
)

const defaultSessionName = "default"

type session struct {
	Name     string
	Windows  int
	Attached bool
}

type sessionManager interface {
	ListSessions() ([]session, error)
	CreateSession(name, cwd string) (session, error)
	KillSession(name string) error
}

type sessionBroker struct {
	mu       sync.Mutex
	sessions map[string]*sessionProcess
}

type sessionProcess struct {
	name     string
	cwd      string
	cmd      *exec.Cmd
	master   *os.File
	buf      bytes.Buffer
	cond     *sync.Cond
	attached bool
	closed   bool
}

func newSessionManager() sessionManager {
	return newSessionBroker()
}

func newSessionBroker() *sessionBroker {
	return &sessionBroker{sessions: map[string]*sessionProcess{}}
}

func (b *sessionBroker) ListSessions() ([]session, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	names := make([]string, 0, len(b.sessions))
	for name := range b.sessions {
		names = append(names, name)
	}
	sort.Strings(names)

	result := make([]session, 0, len(names))
	for _, name := range names {
		proc := b.sessions[name]
		result = append(result, session{
			Name:     proc.name,
			Windows:  1,
			Attached: proc.attached,
		})
	}
	return result, nil
}

func (b *sessionBroker) CreateSession(name, cwd string) (session, error) {
	name = sanitizeSessionName(name)
	if name == "" {
		return session{}, fmt.Errorf("session name is empty")
	}
	if strings.TrimSpace(cwd) == "" {
		if wd, err := os.Getwd(); err == nil {
			cwd = wd
		}
	}
	if strings.TrimSpace(cwd) == "" {
		cwd = "."
	}
	absCwd, err := filepath.Abs(cwd)
	if err == nil {
		cwd = absCwd
	}

	b.mu.Lock()
	if proc, ok := b.sessions[name]; ok && !proc.closed {
		b.mu.Unlock()
		return session{Name: name, Windows: 1, Attached: proc.attached}, nil
	}
	b.mu.Unlock()

	proc, err := startSessionProcess(name, cwd)
	if err != nil {
		return session{}, err
	}

	b.mu.Lock()
	b.sessions[name] = proc
	b.mu.Unlock()

	return session{Name: name, Windows: 1}, nil
}

func (b *sessionBroker) KillSession(name string) error {
	name = sanitizeSessionName(name)
	if name == "" {
		return nil
	}

	b.mu.Lock()
	proc, ok := b.sessions[name]
	if ok {
		delete(b.sessions, name)
	}
	b.mu.Unlock()
	if !ok {
		return nil
	}
	proc.close()
	return nil
}

func (b *sessionBroker) getSession(name string) (*sessionProcess, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	proc, ok := b.sessions[sanitizeSessionName(name)]
	if !ok || proc.closed {
		return nil, false
	}
	return proc, true
}

func (b *sessionBroker) ensureSession(name, cwd string) (*sessionProcess, error) {
	if proc, ok := b.getSession(name); ok {
		return proc, nil
	}
	if _, err := b.CreateSession(name, cwd); err != nil {
		return nil, err
	}
	proc, ok := b.getSession(name)
	if !ok {
		return nil, fmt.Errorf("session %q not available", name)
	}
	return proc, nil
}

func (b *sessionBroker) runSession(name, cwd string) error {
	proc, err := b.ensureSession(name, cwd)
	if err != nil {
		return err
	}

	fd := int(os.Stdin.Fd())
	oldState, err := enterRaw(fd)
	if err != nil {
		return err
	}
	defer leaveRaw(fd, oldState)

	current := proc
	for {
		next, err := current.attachLoop(func() (string, error) {
			if err := leaveRaw(fd, oldState); err != nil {
				return "", err
			}
			selected, runErr := runMenu(b, current.name)
			if _, err := enterRaw(fd); err != nil {
				return "", err
			}
			if runErr != nil {
				return "", runErr
			}
			return selected, nil
		})
		if err != nil {
			return err
		}
		if next == "" {
			return nil
		}
		current, err = b.ensureSession(next, cwd)
		if err != nil {
			return err
		}
	}
}

func startSessionProcess(name, cwd string) (*sessionProcess, error) {
	shell := userShell()
	cmd := exec.Command(shell, "-i", "-l")
	cmd.Dir = cwd
	cmd.Env = os.Environ()
	tty, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}

	proc := &sessionProcess{
		name:   name,
		cwd:    cwd,
		cmd:    cmd,
		master: tty,
	}
	proc.cond = sync.NewCond(&sync.Mutex{})
	go proc.readLoop()
	return proc, nil
}

func (s *sessionProcess) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := s.master.Read(buf)
		if n > 0 {
			s.cond.L.Lock()
			_, _ = s.buf.Write(buf[:n])
			s.cond.Broadcast()
			s.cond.L.Unlock()
		}
		if err != nil {
			s.close()
			return
		}
	}
}

func (s *sessionProcess) attachLoop(menu func() (string, error)) (string, error) {
	s.setAttached(true)
	done := make(chan struct{})
	go s.flushLoop(done)
	defer func() {
		s.setAttached(false)
		s.signal()
		<-done
	}()

	in := make([]byte, 1)
	for {
		n, err := os.Stdin.Read(in)
		if err != nil {
			if err == io.EOF {
				return "", nil
			}
			return "", err
		}
		if n == 0 {
			continue
		}
		if in[0] == 0x06 {
			s.setAttached(false)
			s.signal()
			<-done
			selected, err := menu()
			if err != nil {
				return "", err
			}
			if selected == "" {
				s.setAttached(true)
				done = make(chan struct{})
				go s.flushLoop(done)
				continue
			}
			return selected, nil
		}
		if _, err := s.master.Write(in[:1]); err != nil {
			return "", err
		}
	}
}

func (s *sessionProcess) flushLoop(done chan struct{}) {
	for {
		s.cond.L.Lock()
		for s.buf.Len() == 0 && s.attached && !s.closed {
			s.cond.Wait()
		}
		if !s.attached || s.closed {
			s.cond.L.Unlock()
			close(done)
			return
		}
		data := append([]byte(nil), s.buf.Bytes()...)
		s.buf.Reset()
		s.cond.L.Unlock()
		if len(data) > 0 {
			_, _ = os.Stdout.Write(data)
		}
	}
}

func (s *sessionProcess) setAttached(v bool) {
	s.cond.L.Lock()
	s.attached = v
	s.cond.Broadcast()
	s.cond.L.Unlock()
}

func (s *sessionProcess) signal() {
	s.cond.L.Lock()
	s.cond.Broadcast()
	s.cond.L.Unlock()
}

func (s *sessionProcess) close() {
	s.cond.L.Lock()
	if s.closed {
		s.cond.L.Unlock()
		return
	}
	s.closed = true
	s.attached = false
	s.cond.Broadcast()
	s.cond.L.Unlock()

	if s.master != nil {
		_ = s.master.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
}

func sanitizeSessionName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	var b strings.Builder
	lastDash := false
	for _, r := range name {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case r == '-', r == '_', unicode.IsSpace(r), r == '/', r == '.':
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func userShell() string {
	if shell := strings.TrimSpace(os.Getenv("SHELL")); shell != "" {
		return shell
	}
	return "/bin/sh"
}
