package ui

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"gopkg.in/yaml.v3"

	"github.com/rapsnx/tflow/internal/store"
)

type projectSettingsDocument struct {
	Workdir string `yaml:"workdir"`
}

type projectEditorFinishedMsg struct {
	project  string
	tempPath string
	err      error
}

var (
	activeTempFilesMu sync.Mutex
	activeTempFiles   = make(map[string]struct{})
)

func registerTempFile(path string) {
	activeTempFilesMu.Lock()
	defer activeTempFilesMu.Unlock()
	activeTempFiles[path] = struct{}{}
}

func unregisterTempFile(path string) {
	activeTempFilesMu.Lock()
	defer activeTempFilesMu.Unlock()
	delete(activeTempFiles, path)
}

func cleanupActiveTempFiles() {
	activeTempFilesMu.Lock()
	defer activeTempFilesMu.Unlock()
	for path := range activeTempFiles {
		_ = os.Remove(path)
		delete(activeTempFiles, path)
	}
}

func splitShellWords(cmd string) ([]string, error) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return nil, nil
	}

	var words []string
	var current strings.Builder
	inSingle := false
	inDouble := false
	escaped := false
	inToken := false

	for i := 0; i < len(cmd); i++ {
		ch := cmd[i]

		if escaped {
			current.WriteByte(ch)
			escaped = false
			inToken = true
			continue
		}

		if inSingle {
			if ch == '\'' {
				inSingle = false
			} else {
				current.WriteByte(ch)
			}
			inToken = true
			continue
		}

		if inDouble {
			if ch == '"' {
				inDouble = false
			} else if ch == '\\' {
				if i+1 < len(cmd) && (cmd[i+1] == '"' || cmd[i+1] == '\\' || cmd[i+1] == '$' || cmd[i+1] == '`' || cmd[i+1] == '\n') {
					escaped = true
				} else {
					current.WriteByte(ch)
				}
			} else {
				current.WriteByte(ch)
			}
			inToken = true
			continue
		}

		switch ch {
		case '\\':
			escaped = true
			inToken = true
		case '\'':
			inSingle = true
			inToken = true
		case '"':
			inDouble = true
			inToken = true
		case ' ', '\t', '\n', '\r':
			if inToken {
				words = append(words, current.String())
				current.Reset()
				inToken = false
			}
		default:
			current.WriteByte(ch)
			inToken = true
		}
	}

	if inSingle || inDouble || escaped {
		return nil, fmt.Errorf("unclosed quote or escape in command %q", cmd)
	}

	if inToken {
		words = append(words, current.String())
	}

	return words, nil
}

var resolveEditorCommand = func(tempFile string) (*exec.Cmd, error) {
	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		editor = "nvim"
	}
	parts, err := splitShellWords(editor)
	if err != nil {
		return nil, fmt.Errorf("parse EDITOR: %w", err)
	}
	if len(parts) == 0 {
		parts = []string{"nvim"}
	}
	args := append(parts[1:], tempFile)
	return exec.Command(parts[0], args...), nil
}

func formatProjectSettingsYAML(cfg projectConfig) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("# Project settings for " + cfg.Name + "\n")
	data, err := yaml.Marshal(&projectSettingsDocument{Workdir: cfg.Workdir})
	if err != nil {
		return nil, err
	}
	buf.Write(data)
	return buf.Bytes(), nil
}

func parseProjectSettingsYAML(data []byte) (projectSettingsDocument, error) {
	var doc projectSettingsDocument
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return projectSettingsDocument{}, nil
		}
		return doc, err
	}
	var extra any
	if err := dec.Decode(&extra); err == nil {
		return doc, fmt.Errorf("multiple YAML documents not supported")
	} else if !errors.Is(err, io.EOF) {
		return doc, err
	}
	return doc, nil
}

func (m model) editProject() (tea.Model, tea.Cmd) {
	project := m.contextProject()
	if project == "" {
		m.err = fmt.Errorf("no project selected")
		m.status = "No project selected."
		return m, nil
	}

	cfg := m.projectConfig(project)
	cfg.Name = project

	yamlContent, err := formatProjectSettingsYAML(cfg)
	if err != nil {
		m.err = err
		m.status = fmt.Sprintf("Failed to format settings: %v", err)
		return m, nil
	}

	tempFile, err := os.CreateTemp("", fmt.Sprintf("tflow-project-%s-*.yaml", project))
	if err != nil {
		m.err = err
		m.status = fmt.Sprintf("Failed to create temporary file: %v", err)
		return m, nil
	}
	tempPath := tempFile.Name()
	registerTempFile(tempPath)

	if err := os.Chmod(tempPath, 0600); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		unregisterTempFile(tempPath)
		m.err = err
		m.status = fmt.Sprintf("Failed to set temporary file permissions: %v", err)
		return m, nil
	}

	if _, err := tempFile.Write(yamlContent); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		unregisterTempFile(tempPath)
		m.err = err
		m.status = fmt.Sprintf("Failed to write temporary file: %v", err)
		return m, nil
	}
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		unregisterTempFile(tempPath)
		m.err = err
		m.status = fmt.Sprintf("Failed to close temporary file: %v", err)
		return m, nil
	}

	cmd, err := resolveEditorCommand(tempPath)
	if err != nil {
		_ = os.Remove(tempPath)
		unregisterTempFile(tempPath)
		m.err = err
		m.status = fmt.Sprintf("Failed to resolve editor: %v", err)
		return m, nil
	}

	m.status = ""
	m.err = nil

	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return projectEditorFinishedMsg{
			project:  project,
			tempPath: tempPath,
			err:      err,
		}
	})
}

func (m model) handleProjectEditorFinished(msg projectEditorFinishedMsg) (tea.Model, tea.Cmd) {
	if msg.tempPath != "" {
		defer func() {
			_ = os.Remove(msg.tempPath)
			unregisterTempFile(msg.tempPath)
		}()
	}

	if msg.err != nil {
		m.err = msg.err
		m.status = fmt.Sprintf("Editor error: %v", msg.err)
		return m, nil
	}

	data, err := os.ReadFile(msg.tempPath)
	if err != nil {
		m.err = err
		m.status = fmt.Sprintf("Failed to read settings file: %v", err)
		return m, nil
	}

	doc, err := parseProjectSettingsYAML(data)
	if err != nil {
		m.err = err
		m.status = fmt.Sprintf("Invalid settings: %v", err)
		return m, nil
	}

	cfg := m.projectConfig(msg.project)
	cfg.Name = msg.project
	cfg.Workdir = strings.TrimSpace(doc.Workdir)
	if cfg.Workdir != "" {
		cfg.Workdir = store.NormalizeCWD(cfg.Workdir)
	}

	m.setProjectConfig(cfg)
	if err := m.saveState(); err != nil {
		m.err = err
		m.status = err.Error()
		return m, nil
	}

	if normalizeProjectName(m.sessionProjects[m.currentSession]) == normalizeProjectName(msg.project) {
		m.refreshActiveTopBar()
	}

	m.err = nil
	m.status = ""
	return m, nil
}
