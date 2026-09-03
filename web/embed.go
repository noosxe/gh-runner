package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// Dist returns the embedded filesystem containing static frontend assets.
func Dist() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}
