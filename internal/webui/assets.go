// Package webui embeds the production frontend. Run the frontend build first.
package webui

import (
	"embed"
	"io/fs"
)

//go:embed dist
var assets embed.FS

func Assets() fs.FS {
	result, err := fs.Sub(assets, "dist")
	if err != nil {
		panic(err)
	}
	return result
}
