package core

import (
	"errors"
	"os"
	"sort"
	"time"
)

type FileEntry struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
	IsDir   bool      `json:"is_dir"`
}

// ListFileServerDir returns a flat list of files in the configured file server directory (non-recursive).
func ListFileServerDir() ([]FileEntry, error) {
	cfg, err := LoadConfig()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, errors.New("file server directory not configured")
		}
		return nil, err
	}
	cfg = SanitizeConfig(cfg)

	root := cfg.FileDirectory
	if root == "" {
		return nil, errors.New("file server directory not configured")
	}

	fi, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, errors.New("file server directory path is not a directory")
	}

	entries := []FileEntry{}
	dirEntries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	for _, de := range dirEntries {
		info, err := de.Info()
		if err != nil {
			continue
		}
		entries = append(entries, FileEntry{
			Name:    de.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
			IsDir:   de.IsDir(),
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	return entries, nil
}
