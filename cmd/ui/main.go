package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/alardiians/SwissArmyToolkit/core"
	webassets "github.com/alardiians/SwissArmyToolkit/web"
)

const maxRequestBody = 64 * 1024

var (
	proxyMu  sync.Mutex
	proxyCmd *exec.Cmd

	fileSrvMu sync.Mutex
	fileSrv   *http.Server
)

func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

func handleGetConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	cfg, err := core.LoadConfig()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg = core.DefaultConfig()
		} else {
			respondError(w, http.StatusInternalServerError, "failed to load config")
			return
		}
	}

	cfg = core.SanitizeConfig(cfg)
	respondJSON(w, http.StatusOK, cfg)
}

func handlePostConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	limitedBody := http.MaxBytesReader(w, r.Body, maxRequestBody)
	defer limitedBody.Close()

	dec := json.NewDecoder(limitedBody)
	dec.DisallowUnknownFields()

	var cfg core.Config
	if err := dec.Decode(&cfg); err != nil {
		respondError(w, http.StatusBadRequest, "invalid config payload")
		return
	}

	cfg = core.SanitizeConfig(cfg)
	if err := core.SaveConfig(cfg); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save config")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func handleStartProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	limitedBody := http.MaxBytesReader(w, r.Body, maxRequestBody)
	defer limitedBody.Close()

	proxyMu.Lock()
	defer proxyMu.Unlock()

	if proxyCmd != nil {
		respondError(w, http.StatusConflict, "proxy already running")
		return
	}

	cfg, err := core.LoadConfig()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg = core.DefaultConfig()
		} else {
			respondError(w, http.StatusInternalServerError, "failed to load config")
			return
		}
	}

	cfg = core.SanitizeConfig(cfg)
	cmd, err := core.StartProxy(cfg)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to start proxy")
		return
	}

	proxyCmd = cmd

	respondJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

func handleStopProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	limitedBody := http.MaxBytesReader(w, r.Body, maxRequestBody)
	defer limitedBody.Close()

	proxyMu.Lock()
	defer proxyMu.Unlock()

	if proxyCmd == nil {
		respondJSON(w, http.StatusOK, map[string]string{"status": "not_running"})
		return
	}

	if proxyCmd.Process != nil {
		_ = proxyCmd.Process.Kill()
	}
	go func(cmd *exec.Cmd) {
		_ = cmd.Wait()
	}(proxyCmd)

	proxyCmd = nil
	respondJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	proxyMu.Lock()
	running := proxyCmd != nil
	proxyMu.Unlock()

	respondJSON(w, http.StatusOK, map[string]bool{"proxy_running": running})
}

func handleAgent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	osParam := r.URL.Query().Get("os")
	if osParam != "linux" && osParam != "windows" {
		respondError(w, http.StatusBadRequest, "invalid os")
		return
	}

	cfg, err := core.LoadConfig()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg = core.DefaultConfig()
		} else {
			respondError(w, http.StatusInternalServerError, "failed to load config")
			return
		}
	}

	cfg = core.SanitizeConfig(cfg)

	var cmd string
	if osParam == "linux" {
		cmd = core.AgentCmdLinux(cfg)
	} else {
		cmd = core.AgentCmdWindows(cfg)
	}

	respondJSON(w, http.StatusOK, map[string]string{"command": cmd})
}

func handleSkiddie(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	limitedBody := http.MaxBytesReader(w, r.Body, maxRequestBody)
	defer limitedBody.Close()

	result, err := core.RunSkiddieInstall()
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(strings.ToLower(err.Error()), "linux only") {
			status = http.StatusBadRequest
		}
		respondError(w, status, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

func handleFileConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg, err := core.LoadConfig()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				cfg = core.DefaultConfig()
			} else {
				respondError(w, http.StatusInternalServerError, "failed to load config")
				return
			}
		}
		cfg = core.SanitizeConfig(cfg)
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"file_bind":      cfg.FileBind,
			"file_port":      cfg.FilePort,
			"file_directory": cfg.FileDirectory,
		})
	case http.MethodPost:
		limitedBody := http.MaxBytesReader(w, r.Body, maxRequestBody)
		defer limitedBody.Close()

		dec := json.NewDecoder(limitedBody)
		dec.DisallowUnknownFields()

		type fileCfg struct {
			FileBind      string `json:"file_bind"`
			FilePort      int    `json:"file_port"`
			FileDirectory string `json:"file_directory"`
		}
		var incoming fileCfg
		if err := dec.Decode(&incoming); err != nil {
			respondError(w, http.StatusBadRequest, "invalid file config payload")
			return
		}

		cfg, err := core.LoadConfig()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				cfg = core.DefaultConfig()
			} else {
				respondError(w, http.StatusInternalServerError, "failed to load config")
				return
			}
		}

		cfg.FileBind = incoming.FileBind
		cfg.FilePort = incoming.FilePort
		cfg.FileDirectory = incoming.FileDirectory
		cfg = core.SanitizeConfig(cfg)

		if err := core.SaveConfig(cfg); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to save config")
			return
		}

		respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func handleFileStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	limitedBody := http.MaxBytesReader(w, r.Body, maxRequestBody)
	defer limitedBody.Close()

	fileSrvMu.Lock()
	defer fileSrvMu.Unlock()

	if fileSrv != nil {
		respondError(w, http.StatusConflict, "file server already running")
		return
	}

	cfg, err := core.LoadConfig()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg = core.DefaultConfig()
		} else {
			respondError(w, http.StatusInternalServerError, "failed to load config")
			return
		}
	}
	cfg = core.SanitizeConfig(cfg)

	if cfg.FileDirectory == "" {
		respondError(w, http.StatusBadRequest, "invalid file directory")
		return
	}
	info, statErr := os.Stat(cfg.FileDirectory)
	if statErr != nil || !info.IsDir() {
		respondError(w, http.StatusBadRequest, "invalid file directory")
		return
	}

	addr := fmt.Sprintf("%s:%d", cfg.FileBind, cfg.FilePort)
	fs := http.FileServer(http.Dir(cfg.FileDirectory))

	srv := &http.Server{
		Addr:         addr,
		Handler:      fs,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to start file server")
		return
	}

	fileSrv = srv

	go func(s *http.Server, l net.Listener) {
		if err := s.Serve(l); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("file server error: %v", err)
		}
	}(srv, ln)

	respondJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

func handleFileStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	limitedBody := http.MaxBytesReader(w, r.Body, maxRequestBody)
	defer limitedBody.Close()

	fileSrvMu.Lock()
	defer fileSrvMu.Unlock()

	if fileSrv == nil {
		respondJSON(w, http.StatusOK, map[string]string{"status": "not_running"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := fileSrv.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("file server shutdown error: %v", err)
	}
	fileSrv = nil

	respondJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func handleFileStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	fileSrvMu.Lock()
	running := fileSrv != nil
	fileSrvMu.Unlock()

	respondJSON(w, http.StatusOK, map[string]bool{"file_server_running": running})
}

func handleFileCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	osParam := r.URL.Query().Get("os")
	if osParam != "linux" && osParam != "windows" {
		respondError(w, http.StatusBadRequest, "invalid os")
		return
	}

	filename := strings.TrimSpace(r.URL.Query().Get("filename"))
	if filename == "" || strings.Contains(filename, "/") || strings.Contains(filename, "\\") || strings.Contains(filename, "..") {
		respondError(w, http.StatusBadRequest, "invalid filename")
		return
	}

	cfg, err := core.LoadConfig()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg = core.DefaultConfig()
		} else {
			respondError(w, http.StatusInternalServerError, "failed to load config")
			return
		}
	}
	cfg = core.SanitizeConfig(cfg)

	url := fmt.Sprintf("http://%s:%d/%s", cfg.PublicIP, cfg.FilePort, filename)

	var cmd string
	if osParam == "linux" {
		cmd = fmt.Sprintf("curl -o %s %s", filename, url)
	} else {
		cmd = fmt.Sprintf(`powershell -Command "Invoke-WebRequest -Uri '%s' -OutFile '%s'"`, url, filename)
	}

	respondJSON(w, http.StatusOK, map[string]string{"command": cmd})
}

func handleFileList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	entries, err := core.ListFileServerDir()
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, entries)
}

func handleFSScout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	defer r.Body.Close()

	var req core.FSScoutRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	res, err := core.RunFSScout(req)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, res)
		return
	}

	respondJSON(w, http.StatusOK, res)
}

func main() {
	embedded, err := webassets.FS()
	if err != nil {
		log.Fatalf("failed to load embedded web assets: %v", err)
	}

	if _, err := core.InitLootDir(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize loot directory: %v\n", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			handleGetConfig(w, r)
			return
		}
		if r.Method == http.MethodPost {
			handlePostConfig(w, r)
			return
		}
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	})
	mux.HandleFunc("/api/start-proxy", handleStartProxy)
	mux.HandleFunc("/api/stop-proxy", handleStopProxy)
	mux.HandleFunc("/api/status", handleStatus)
	mux.HandleFunc("/api/agent", handleAgent)
	mux.HandleFunc("/api/file-config", handleFileConfig)
	mux.HandleFunc("/api/file-start", handleFileStart)
	mux.HandleFunc("/api/file-stop", handleFileStop)
	mux.HandleFunc("/api/file-status", handleFileStatus)
	mux.HandleFunc("/api/file-command", handleFileCommand)
	mux.HandleFunc("/api/file-list", handleFileList)
	mux.HandleFunc("/api/fs-scout", handleFSScout)
	mux.HandleFunc("/api/skiddie", handleSkiddie)

	// Serve assets: prefer app data dir, fallback to embedded root.
	assetDir := ""
	if base, err := core.DefaultAppDataDir(); err == nil {
		assetDir = filepath.Join(base, "assets")
	}
	if assetDir != "" {
		mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir(assetDir))))
	}

	// Serve embedded index (SPA shell).
	staticFS := http.FileServer(http.FS(embedded))
	mux.Handle("/", staticFS)

	srv := &http.Server{
		Addr:              "127.0.0.1:8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Println("PivotOnTheGO UI listening on 127.0.0.1:8080")
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server error: %v", err)
	}
}
