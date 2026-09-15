package store

const (
	SessionTypeTerminal = "terminal"
	SessionTypeGit      = "git"
	SessionTypeAgent    = "agent"
)

type AppState struct {
	Projects []Project
}

type Project struct {
	Name        string
	Workdir     string
	AgentBinary string
	Sessions    []PersistentSession
}

type PersistentSession struct {
	ID      string
	Label   string
	Type    string
	Command string
}

type storedState struct {
	Projects []storedProject `json:"projects"`
}

type storedProject struct {
	Name        string                    `json:"name"`
	Workdir     string                    `json:"workdir"`
	AgentBinary string                    `json:"agentBinary,omitempty"`
	Sessions    []storedPersistentSession `json:"sessions"`
}

type storedPersistentSession struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Type    string `json:"type,omitempty"`
	Command string `json:"command,omitempty"`
}
