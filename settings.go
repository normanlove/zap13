package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type settings struct {
	Sliders []int `json:"sliders"`
}

func settingsPath() string {
	return filepath.Join(os.Getenv("APPDATA"), "zap13", "settings.json")
}

func loadSliders() limits {
	var s settings
	b, err := os.ReadFile(settingsPath())
	if err != nil || json.Unmarshal(b, &s) != nil || len(s.Sliders) != 3 {
		return usbcLimits
	}
	return limits{s.Sliders[0], s.Sliders[1], s.Sliders[2]}.clamp()
}

func saveSliders(l limits) {
	p := settingsPath()
	os.MkdirAll(filepath.Dir(p), 0o755)
	b, _ := json.Marshal(settings{Sliders: l[:]})
	os.WriteFile(p, b, 0o644)
}
