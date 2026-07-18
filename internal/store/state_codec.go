package store

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func emptyAppState() AppState {
	return AppState{
		Projects:        []string{},
		SessionProjects: map[string]string{},
		SessionLabels:   map[string]string{},
		ProjectConfigs:  map[string]ProjectConfig{},
	}
}

func encodeAppState(state AppState) ([]byte, error) {
	stored := storedState{
		ProjectOrder:    append([]string(nil), state.Projects...),
		Projects:        make(map[string]storedProject, len(state.Projects)),
		SessionProjects: cloneStringMap(state.SessionProjects),
		SessionLabels:   cloneStringMap(state.SessionLabels),
	}
	for _, project := range state.Projects {
		stored.Projects[project] = storedProject{Workdir: state.ProjectConfigs[project].Workdir}
	}
	return json.MarshalIndent(stored, "", "  ")
}

func decodeAppState(data []byte) (AppState, error) {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
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
	state.Projects = append(state.Projects, stored.ProjectOrder...)
	for project, cfg := range stored.Projects {
		state.ProjectConfigs[project] = ProjectConfig{Name: project, Workdir: cfg.Workdir}
	}
	state.SessionProjects = cloneStringMap(stored.SessionProjects)
	state.SessionLabels = cloneStringMap(stored.SessionLabels)
	return NormalizeAppState(state), nil
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return map[string]string{}
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
