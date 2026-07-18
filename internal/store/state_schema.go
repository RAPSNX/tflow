package store

type AppState struct {
	Projects        []string
	SessionProjects map[string]string
	SessionLabels   map[string]string
	ProjectConfigs  map[string]ProjectConfig
}

type storedState struct {
	ProjectOrder    []string                 `json:"project_order"`
	Projects        map[string]storedProject `json:"projects"`
	SessionProjects map[string]string        `json:"session_projects"`
	SessionLabels   map[string]string        `json:"session_labels"`
}

type storedProject struct {
	Workdir string `json:"workdir"`
}
