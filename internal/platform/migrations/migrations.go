// Package migrations holds the app migrations that no feature owns.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
