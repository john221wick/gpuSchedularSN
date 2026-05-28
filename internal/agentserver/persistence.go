package agentserver

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type PersistedJob struct {
	ID        string `json:"id"`
	Command   string `json:"command"`
	GPUIDs    []int  `json:"gpuIDs"`
	PID       int    `json:"pid"`
	Status    string `json:"status"`
	StartedAt string `json:"startedAt"`
	WorkDir   string `json:"workDir"`
}

type PersistedState struct {
	Jobs []PersistedJob `json:"jobs"`
}

func LoadState(dir string) (*PersistedState, error) {
	path := filepath.Join(dir, "state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &PersistedState{}, nil
		}
		return nil, err
	}

	var state PersistedState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func SaveState(dir string, state *PersistedState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	tmpPath := filepath.Join(dir, "state.json.tmp")
	finalPath := filepath.Join(dir, "state.json")

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}

	return os.Rename(tmpPath, finalPath)
}
