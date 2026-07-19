package store

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func emptyAppState() AppState {
	return AppState{Projects: []Project{}}
}

func encodeAppState(state AppState) ([]byte, error) {
	state = NormalizeAppState(state)
	stored := storedState{Projects: make([]storedProject, 0, len(state.Projects))}
	for _, project := range state.Projects {
		encodedProject := storedProject{Name: project.Name, Workdir: project.Workdir, Sessions: make([]storedPersistentSession, 0, len(project.Sessions))}
		for _, session := range project.Sessions {
			encodedProject.Sessions = append(encodedProject.Sessions, storedPersistentSession{ID: session.ID, Label: session.Label})
		}
		stored.Projects = append(stored.Projects, encodedProject)
	}
	return json.MarshalIndent(stored, "", "  ")
}

func decodeAppState(data []byte) (AppState, error) {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	var stored storedState
	if err := decoder.Decode(&stored); err != nil {
		return AppState{}, fmt.Errorf("invalid state: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return AppState{}, fmt.Errorf("invalid state: expected one JSON object")
		}
		return AppState{}, fmt.Errorf("invalid state: %w", err)
	}
	state := emptyAppState()
	for _, project := range stored.Projects {
		loaded := Project{Name: project.Name, Workdir: project.Workdir, Sessions: make([]PersistentSession, 0, len(project.Sessions))}
		for _, session := range project.Sessions {
			loaded.Sessions = append(loaded.Sessions, PersistentSession{ID: session.ID, Label: session.Label})
		}
		state.Projects = append(state.Projects, loaded)
	}
	return NormalizeAppState(state), nil
}
