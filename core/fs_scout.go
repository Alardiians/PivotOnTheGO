package core

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type FSScoutProtocol string

const (
	FSProtocolSSH       FSScoutProtocol = "ssh"
	FSProtocolSMB       FSScoutProtocol = "smb"
	FSProtocolFTP       FSScoutProtocol = "ftp"
	FSProtocolEvilWinRM FSScoutProtocol = "evil-winrm"
)

type FSScoutMode string

const (
	FSModeFast    FSScoutMode = "fast"
	FSModeStealth FSScoutMode = "stealth"
)

type FSScoutRequest struct {
	Protocol FSScoutProtocol `json:"protocol"`
	Host     string          `json:"host"`
	Port     int             `json:"port"`
	Username string          `json:"username"`
	Password string          `json:"password"`

	SMBShare string `json:"smb_share"`

	StartDir string      `json:"start_dir"`
	Depth    int         `json:"depth"`
	Mode     FSScoutMode `json:"mode"`
}

type FSScoutResult struct {
	OutputFile string `json:"output_file"`
	Protocol   string `json:"protocol"`
	Mode       string `json:"mode"`
	Host       string `json:"host"`
	Error      string `json:"error,omitempty"`
}

func RunFSScout(req FSScoutRequest) (FSScoutResult, error) {
	if req.Host == "" {
		return FSScoutResult{}, errors.New("host is required")
	}
	if req.Username == "" || req.Password == "" {
		return FSScoutResult{}, errors.New("username and password are required")
	}
	if req.StartDir == "" {
		return FSScoutResult{}, errors.New("start directory is required")
	}
	if req.Depth <= 0 {
		req.Depth = 3
	}
	if req.Mode == "" {
		req.Mode = FSModeFast
	}

	lootDir, err := DefaultLootDir()
	if err != nil {
		return FSScoutResult{}, err
	}

	fsBase := filepath.Join(lootDir, "fs", sanitizeHost(req.Host))
	if err := os.MkdirAll(fsBase, 0o755); err != nil {
		return FSScoutResult{}, err
	}

	ts := time.Now().Format("2006-01-02_15-04-05")
	outName := fmt.Sprintf("%s_%s_%s.txt", ts, req.Protocol, req.Mode)
	outPath := filepath.Join(fsBase, outName)

	var runErr error
	switch req.Protocol {
	case FSProtocolSSH:
		runErr = runFSScoutSSH(req, outPath)
	case FSProtocolSMB:
		runErr = runFSScoutSMB(req, outPath)
	case FSProtocolFTP:
		runErr = errors.New("FTP auto-scout not implemented yet; use generate-only / manual mode")
	case FSProtocolEvilWinRM:
		runErr = runFSScoutEvilWinRM(req, outPath)
	default:
		runErr = errors.New("unsupported protocol")
	}

	res := FSScoutResult{
		OutputFile: outPath,
		Protocol:   string(req.Protocol),
		Mode:       string(req.Mode),
		Host:       req.Host,
	}
	if runErr != nil {
		res.Error = runErr.Error()
		return res, runErr
	}

	return res, nil
}

func sanitizeHost(h string) string {
	h = strings.TrimSpace(h)
	h = strings.ReplaceAll(h, ":", "_")
	h = strings.ReplaceAll(h, "/", "_")
	return h
}

func runFSScoutSSH(req FSScoutRequest, outPath string) error {
	port := req.Port
	if port == 0 {
		port = 22
	}
	depthStr := fmt.Sprintf("%d", req.Depth)
	target := fmt.Sprintf("%s@%s", req.Username, req.Host)

	args := []string{
		"-p", fmt.Sprintf("%d", port),
		target,
		"find", req.StartDir,
		"-maxdepth", depthStr,
		"-type", "f",
		"-printf", "%p\n",
	}

	cmd := exec.Command("ssh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	writeErr := writeFSScoutOutputSSH(outPath, stdout.String(), stderr.String())
	if err != nil {
		return fmt.Errorf("ssh command failed: %w", err)
	}
	return writeErr
}

func writeFSScoutOutputSSH(outPath, stdout, stderr string) error {
	var buf bytes.Buffer

	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		buf.WriteString("FILE|")
		buf.WriteString(line)
		buf.WriteByte('\n')
	}

	for _, line := range strings.Split(stderr, "\n") {
		if strings.Contains(line, "Permission denied") {
			buf.WriteString("DENIED|")
			buf.WriteString(strings.TrimSpace(line))
			buf.WriteByte('\n')
		}
	}

	return os.WriteFile(outPath, buf.Bytes(), 0o644)
}

func runFSScoutSMB(req FSScoutRequest, outPath string) error {
	if req.SMBShare == "" {
		return errors.New("SMB share name is required for smb protocol")
	}

	target := fmt.Sprintf("//%s/%s", req.Host, req.SMBShare)
	userArg := fmt.Sprintf("%s%%%s", req.Username, req.Password)

	args := []string{
		target,
		"-U", userArg,
		"-c", "recurse; ls",
	}

	cmd := exec.Command("smbclient", args...)
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf

	err := cmd.Run()
	parseErr := parseAndWriteSMBOutput(outPath, outBuf.String())
	if err != nil {
		return fmt.Errorf("smbclient command failed: %w", err)
	}
	return parseErr
}

func parseAndWriteSMBOutput(outPath, raw string) error {
	var buf bytes.Buffer
	lines := strings.Split(raw, "\n")

	for _, line := range lines {
		l := strings.TrimSpace(line)
		if l == "" {
			continue
		}

		if strings.Contains(l, "NT_STATUS_ACCESS_DENIED") {
			buf.WriteString("DENIED|")
			buf.WriteString(l)
			buf.WriteByte('\n')
			continue
		}

		fields := strings.Fields(l)
		if len(fields) > 0 {
			name := fields[0]
			if name != "." && name != ".." {
				buf.WriteString("FILE|")
				buf.WriteString(name)
				buf.WriteByte('\n')
			}
		}
	}

	return os.WriteFile(outPath, buf.Bytes(), 0o644)
}

func runFSScoutEvilWinRM(req FSScoutRequest, outPath string) error {
	port := req.Port
	if port == 0 {
		port = 5985
	}

	psScript := fmt.Sprintf(`
$start = "%s"
$depth = %d
function Walk($path, $level) {
    if ($level -gt $depth) { return }
    try {
        Get-ChildItem -Path $path -ErrorAction Stop | ForEach-Object {
            if ($_.PSIsContainer) {
                Walk $_.FullName ($level + 1)
            } else {
                "FILE|$($_.FullName)"
            }
        }
    } catch {
        "DENIED|$path"
    }
}
Walk $start 0
`, req.StartDir, req.Depth)
	psScript = strings.ReplaceAll(psScript, "\n", " ")

	args := []string{
		"-i", req.Host,
		"-u", req.Username,
		"-p", req.Password,
		"-P", fmt.Sprintf("%d", port),
		"-c", psScript,
	}

	cmd := exec.Command("evil-winrm", args...)
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf

	err := cmd.Run()
	parseErr := parseAndWriteFSOutputGeneric(outPath, outBuf.String())
	if err != nil {
		return fmt.Errorf("evil-winrm command failed: %w", err)
	}
	return parseErr
}

func parseAndWriteFSOutputGeneric(outPath, raw string) error {
	var buf bytes.Buffer
	for _, line := range strings.Split(raw, "\n") {
		l := strings.TrimSpace(line)
		if l == "" {
			continue
		}
		if strings.HasPrefix(l, "FILE|") || strings.HasPrefix(l, "DENIED|") {
			buf.WriteString(l)
			buf.WriteByte('\n')
		}
	}
	return os.WriteFile(outPath, buf.Bytes(), 0o644)
}
