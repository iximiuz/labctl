package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ponytail: two tiny JSON files in $HOME, best-effort. Theme lives in
// ~/.labctl.config; per-lab region cache + live forwards in ~/.labctl.rc.

func homePath(name string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, name)
}

func loadJSON(path string, v any) {
	if path == "" {
		return
	}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, v)
	}
}

func saveJSON(path string, v any) {
	if path == "" {
		return
	}
	if data, err := json.MarshalIndent(v, "", "  "); err == nil {
		_ = os.WriteFile(path, data, 0o600)
	}
}

type uiConfig struct {
	Theme string `json:"theme,omitempty"`
}

func loadUIConfig() uiConfig {
	var c uiConfig
	loadJSON(homePath(".labctl.config"), &c)
	return c
}

func (c uiConfig) save() { saveJSON(homePath(".labctl.config"), c) }

type savedForward struct {
	PlayID   string `json:"playID"`
	PlayName string `json:"playName"`
	Spec     string `json:"spec"`
}

type uiState struct {
	Regions  map[string]string `json:"regions,omitempty"`
	Forwards []savedForward    `json:"forwards,omitempty"`
}

func loadUIState() uiState {
	s := uiState{Regions: map[string]string{}}
	loadJSON(homePath(".labctl.rc"), &s)
	if s.Regions == nil {
		s.Regions = map[string]string{}
	}
	return s
}

func (s uiState) save() { saveJSON(homePath(".labctl.rc"), s) }
