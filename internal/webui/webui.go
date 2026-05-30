package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var files embed.FS

func FS() fs.FS {
	f, err := fs.Sub(files, "dist")
	if err != nil {
		panic(err)
	}
	return f
}
