// Command gx is the Gx command line tool of this project. Run it with
// "go run ./cmd/gx <command>", so every developer uses the Gx version in
// go.mod.
package main

import (
	"os"

	"github.com/alternayte/gx/gxcli"
)

func main() {
	// To add a plugin, give it to Main as a second argument:
	// gxcli.WithPlugins(chartzoom.Plugin()). The Go compiler checks each
	// plugin, and no code loads at run time.
	os.Exit(gxcli.Main(os.Args[1:]))
}
