package core

import (
	"fmt"
	"os/exec"
	"strings"
)

// AgentCmdLinux returns the command to run the agent on Linux.
func AgentCmdLinux(cfg Config) string {
	cfg = SanitizeConfig(cfg)
	return fmt.Sprintf("./%s -connect %s:%d -ignore-cert", cfg.AgentBinary, cfg.PublicIP, cfg.ProxyPort)
}

// AgentCmdWindows returns a PowerShell one-liner to run the agent on Windows.
func AgentCmdWindows(cfg Config) string {
	cfg = SanitizeConfig(cfg)

	binary := cfg.AgentBinary
	if !strings.HasSuffix(strings.ToLower(binary), ".exe") {
		binary += ".exe"
	}

	return fmt.Sprintf(`powershell -Command "Start-Process -FilePath .\\%s -ArgumentList '-connect %s:%d -ignore-cert'"`, binary, cfg.PublicIP, cfg.ProxyPort)
}

// StartProxy launches the ligolo proxy with the provided configuration.
func StartProxy(cfg Config) (*exec.Cmd, error) {
	cfg = SanitizeConfig(cfg)
	addr := fmt.Sprintf("%s:%d", cfg.ProxyBind, cfg.ProxyPort)

	cmd := exec.Command(cfg.ProxyBinary, "-laddr", addr, "-selfcert")
	return cmd, cmd.Start()
}
