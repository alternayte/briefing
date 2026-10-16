//go:build gxdev

package main

import (
	"github.com/alternayte/briefing/gxdev_gallery"
	"github.com/alternayte/briefing/gxdev_symbols"
	"github.com/alternayte/gx"
)

// setupGallery installs the component fixtures of the dev gallery at
// /_gx/gallery, and the symbol table that lets gx dev put a changed
// template into the running app with no rebuild. The file is in a dev
// build only.
func setupGallery() {
	gx.SetGallery(gxdev_gallery.Fixtures())
	gx.SetDevSymbols(gxdev_symbols.Packages())
}
