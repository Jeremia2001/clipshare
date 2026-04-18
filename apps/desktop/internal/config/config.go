package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	APIURL          string   `json:"api_url"`
	AccountUsername string   `json:"account_username,omitempty"`
	Theme           string   `json:"theme"`
	AutoStart       bool     `json:"auto_start"`
	WatchFolders    []string `json:"watch_folders"`
	DevMode         bool     `json:"dev_mode"`
}

func getConfigPath() string {
	configDir, _ := os.UserConfigDir()
	return filepath.Join(configDir, "clipshare", "config.json")
}

func Load() (*Config, error) {
	configPath := getConfigPath()

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return nil, err
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return &Config{
			APIURL:       "http://127.0.0.1:8080",
			Theme:        "dark",
			AutoStart:    false,
			WatchFolders: []string{},
			DevMode:      os.Getenv("ENV") == "development" || os.Getenv("CLIPSHARE_DEV") == "1",
		}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	if os.Getenv("ENV") == "development" || os.Getenv("CLIPSHARE_DEV") == "1" {
		config.DevMode = true
	}

	return &config, nil
}

func Save(config *Config) error {
	configPath := getConfigPath()

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}
