// Package route holds the route types of the admin slice.
package route

import "github.com/alternayte/gx"

// Admin is the operations page.
type Admin struct {
	gx.Route `GET /admin`
}

// ToggleSource turns polling of one source on or off.
type ToggleSource struct {
	gx.Route `POST /admin/sources/{ID}/toggle`
	ID       int64
}

func (in *ToggleSource) Rules() gx.Rules { return gx.Rules{} }
