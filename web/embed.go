package web

import (
	"embed"
	"io/fs"
)

// files holds the embedded web UI assets (index and static content).
//
//go:embed index.html
var files embed.FS

// FS returns the embedded web filesystem.
func FS() (fs.FS, error) {
	return fs.Sub(files, ".")
}
