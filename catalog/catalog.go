// Package catalog embeds the source catalog that the seed command loads.
package catalog

import _ "embed"

//go:embed sources.yaml
var Sources []byte
