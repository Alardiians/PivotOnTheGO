package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultProxyBind   = "127.0.0.1"
	defaultProxyPort   = 11601
	defaultPublicIP    = "CHANGEME_PUBLIC_IP"
	defaultProxyBinary = "/opt/ligolo/proxy"
	defaultAgentBinary = "agent"
)

// Config holds settings for the PivotOnTheGO wrapper.
type Config struct {
	ProxyBind   string `json:"proxy_bind"`
	ProxyPort   int    `json:"proxy_port"`
	PublicIP    string `json:"public_ip"`
	ProxyBinary string `json:"proxy_binary"`
	AgentBinary string `json:"agent_binary"`

	FileBind      string `json:"file_bind"`
	FilePort      int    `json:"file_port"`
	FileDirectory string `json:"file_directory"`
}

// DefaultConfig returns a configuration populated with safe defaults.
func DefaultConfig() Config {
	return Config{
		ProxyBind:     defaultProxyBind,
		ProxyPort:     defaultProxyPort,
		PublicIP:      defaultPublicIP,
		ProxyBinary:   defaultProxyBinary,
		AgentBinary:   defaultAgentBinary,
		FileBind:      "0.0.0.0",
		FilePort:      8000,
		FileDirectory: "",
	}
}

// ConfigPath returns the config file location in the user's home directory.
func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	newPath := filepath.Join(home, ".config", "PivotOnTheGO", "config.json")
	oldPath := filepath.Join(home, ".config", "SwissArmyToolkit", "config.json")

	// Prefer new path; fall back to legacy if it already exists.
	if _, err := os.Stat(newPath); err == nil {
		return newPath, nil
	}
	if _, err := os.Stat(oldPath); err == nil {
		return oldPath, nil
	}
	return newPath, nil
}

// SanitizeConfig trims and validates configuration values, applying defaults when needed.
func SanitizeConfig(cfg Config) Config {
	cfg.ProxyBind = strings.TrimSpace(cfg.ProxyBind)
	cfg.PublicIP = strings.TrimSpace(cfg.PublicIP)
	cfg.ProxyBinary = strings.TrimSpace(cfg.ProxyBinary)
	cfg.AgentBinary = strings.TrimSpace(cfg.AgentBinary)
	cfg.FileBind = strings.TrimSpace(cfg.FileBind)
	cfg.FileDirectory = strings.TrimSpace(cfg.FileDirectory)

	oldAppData := LegacyAppDataDirPath()
	newAppData, _ := DefaultAppDataDir()
	oldLoot := filepath.Join(oldAppData, "loot")
	newLoot := filepath.Join(newAppData, "loot")
	oldProxy := filepath.Join(oldAppData, "ligolo", "proxy")
	newProxy := filepath.Join(newAppData, "ligolo", "proxy")

	if cfg.ProxyPort <= 0 || cfg.ProxyPort > 65535 {
		cfg.ProxyPort = defaultProxyPort
	}
	if cfg.ProxyBind == "" {
		cfg.ProxyBind = defaultProxyBind
	}
	if cfg.ProxyBinary == "" {
		cfg.ProxyBinary = defaultProxyBinary
	}
	if cfg.ProxyBinary == oldProxy {
		if _, err := os.Stat(newProxy); err == nil {
			cfg.ProxyBinary = newProxy
		}
	}
	if cfg.AgentBinary == "" {
		cfg.AgentBinary = defaultAgentBinary
	}
	if cfg.FilePort <= 0 || cfg.FilePort > 65535 {
		cfg.FilePort = 8000
	}
	if cfg.FileBind == "" {
		cfg.FileBind = "0.0.0.0"
	}
	if cfg.FileDirectory == "" {
		if lootDir, err := InitLootDir(); err == nil {
			cfg.FileDirectory = lootDir
		}
	} else if cfg.FileDirectory == oldLoot {
		if _, err := os.Stat(newLoot); err == nil {
			cfg.FileDirectory = newLoot
		}
	}
	return cfg
}

// LoadConfig reads the configuration file if it exists, or returns defaults.
// If the file is missing, it returns DefaultConfig and os.ErrNotExist.
func LoadConfig() (Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return DefaultConfig(), err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), os.ErrNotExist
		}
		return DefaultConfig(), err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return DefaultConfig(), err
	}
	return SanitizeConfig(cfg), nil
}

// SaveConfig writes the configuration to disk after sanitizing it.
func SaveConfig(cfg Config) error {
	cfg = SanitizeConfig(cfg)

	path, err := ConfigPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}
