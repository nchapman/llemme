package web

import (
	"embed"
	"io/fs"
)

//go:embed all:out
var content embed.FS

// DistFS returns the embedded static files as an fs.FS rooted at the out/ directory.
func DistFS() (fs.FS, error) {
	return fs.Sub(content, "out")
}
