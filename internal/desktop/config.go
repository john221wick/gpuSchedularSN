package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// SavedNode persists node connection info across app restarts.
type SavedNode struct {
	ID         string `json:"id"`
	SSHCommand string `json:"sshCommand"`
	KeyPath    string `json:"keyPath"`
	MockMode   bool   `json:"mockMode"`
	LocalDir   string `json:"localDir"`
	RemoteDir  string `json:"remoteDir"`
}

// DesktopConfig persists desktop app state.
type DesktopConfig struct {
	RemoteMode bool        `json:"remoteMode"`
	Nodes      []SavedNode `json:"nodes"`
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".gpusched", "desktop-config.json")
}

func LoadDesktopConfig() (*DesktopConfig, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &DesktopConfig{}, nil
		}
		return nil, err
	}
	var cfg DesktopConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return &DesktopConfig{}, nil // corrupt file, start fresh
	}
	return &cfg, nil
}

func SaveDesktopConfig(cfg *DesktopConfig) error {
	dir := filepath.Dir(configPath())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := configPath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, configPath())
}
