package core

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
)

const (
	// Ligolo-ng v0.8.2 download URLs for Linux amd64.
	LigoloVersion            = "v0.8.2"
	LigoloProxyURLLinuxAmd64 = "https://github.com/nicocha30/ligolo-ng/releases/download/v0.8.2/ligolo-ng_proxy_0.8.2_linux_amd64.tar.gz"
	LigoloAgentURLLinuxAmd64 = "https://github.com/nicocha30/ligolo-ng/releases/download/v0.8.2/ligolo-ng_agent_0.8.2_linux_amd64.tar.gz"
)

// LigoloInstallDir returns the default install directory for ligolo binaries.
func LigoloInstallDir() (string, error) {
	base, err := DefaultAppDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "ligolo"), nil
}

// LigoloStatus indicates whether ligolo binaries are present and configured.
type LigoloStatus struct {
	Installed  bool   `json:"installed"`
	ProxyPath  string `json:"proxy_path"`
	AgentName  string `json:"agent_name"`
	InstallDir string `json:"install_dir"`
	Reason     string `json:"reason,omitempty"`
}

// CheckLigoloInstalled checks config and filesystem to see if ligolo is ready.
func CheckLigoloInstalled() (LigoloStatus, error) {
	cfg, err := LoadConfig()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return LigoloStatus{}, err
	}
	cfg = SanitizeConfig(cfg)

	installDir, err := LigoloInstallDir()
	if err != nil {
		return LigoloStatus{}, err
	}

	status := LigoloStatus{
		Installed:  false,
		ProxyPath:  cfg.ProxyBinary,
		AgentName:  cfg.AgentBinary,
		InstallDir: installDir,
	}

	if cfg.ProxyBinary == "" {
		status.Reason = "proxy binary path not set"
		return status, nil
	}

	if _, err := os.Stat(cfg.ProxyBinary); err != nil {
		status.Reason = "proxy binary not found on disk"
		return status, nil
	}

	agentPath := filepath.Join(installDir, cfg.AgentBinary)
	if _, err := os.Stat(agentPath); err != nil {
		status.Reason = "agent binary not found in install dir"
		return status, nil
	}

	status.Installed = true
	status.Reason = ""
	return status, nil
}

func downloadAndExtractTarGz(url, destDir, destFilename string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)

	targetPath := filepath.Join(destDir, destFilename)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		if filepath.Base(hdr.Name) != destFilename {
			continue
		}

		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return err
		}

		out, err := os.Create(targetPath)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return err
		}
		out.Close()
		if err := os.Chmod(targetPath, 0o755); err != nil {
			return err
		}
		return nil
	}

	return fmt.Errorf("file %s not found in tar.gz", destFilename)
}

// SkiddieResult provides installer outcome details.
type SkiddieResult struct {
	InstalledBefore bool   `json:"installed_before"`
	ProxyPath       string `json:"proxy_path"`
	AgentName       string `json:"agent_name"`
	Message         string `json:"message"`
}

// RunSkiddieInstall installs ligolo binaries for Linux and updates config.
func RunSkiddieInstall() (SkiddieResult, error) {
	if runtime.GOOS != "linux" {
		return SkiddieResult{}, errors.New("Skiddie Mode is supported on Linux only")
	}

	status, err := CheckLigoloInstalled()
	if err != nil {
		return SkiddieResult{}, err
	}

	result := SkiddieResult{
		InstalledBefore: status.Installed,
		ProxyPath:       status.ProxyPath,
		AgentName:       status.AgentName,
	}

	if status.Installed {
		result.Message = "Ligolo-ng already installed and configured."
		return result, nil
	}

	installDir := status.InstallDir
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return result, err
	}

	proxyPath := filepath.Join(installDir, "proxy")
	agentPath := filepath.Join(installDir, "agent")

	if err := downloadAndExtractTarGz(LigoloProxyURLLinuxAmd64, installDir, "proxy"); err != nil {
		return result, fmt.Errorf("failed to install proxy: %w", err)
	}
	if err := downloadAndExtractTarGz(LigoloAgentURLLinuxAmd64, installDir, "agent"); err != nil {
		return result, fmt.Errorf("failed to install agent: %w", err)
	}

	if err := os.Chmod(proxyPath, 0o755); err != nil {
		return result, err
	}
	if err := os.Chmod(agentPath, 0o755); err != nil {
		return result, err
	}

	cfg, err := LoadConfig()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return result, err
	}
	cfg.ProxyBinary = proxyPath
	cfg.AgentBinary = "agent"
	cfg = SanitizeConfig(cfg)

	if err := SaveConfig(cfg); err != nil {
		return result, err
	}

	result.ProxyPath = proxyPath
	result.AgentName = "agent"
	result.Message = "Ligolo-ng installed and config updated."
	return result, nil
}
