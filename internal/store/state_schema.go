package store

type AppState struct {
	Projects []Project
}

type Project struct {
	Name     string
	Workdir  string
	Sessions []PersistentSession
}

type PersistentSession struct {
	ID    string
	Label string
}

type storedState struct {
	Projects []storedProject `json:"projects"`
}

type storedProject struct {
	Name     string                    `json:"name"`
	Workdir  string                    `json:"workdir"`
	Sessions []storedPersistentSession `json:"sessions"`
}

type storedPersistentSession struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}
