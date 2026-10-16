// Package route holds the route types of the sources slice.
package route

import "github.com/alternayte/gx"

// List lists the sources the reader can follow.
type List struct {
	gx.Route `GET /api/sources`
}

// Subscribe follows or unfollows one source.
type Subscribe struct {
	gx.Route `POST /sources/{SourceID}/subscribe`
	SourceID int64
	On       bool
}

func (in *Subscribe) Rules() gx.Rules { return gx.Rules{} }

// AddRSS adds an RSS or Atom feed and follows it. Address is the feed URL;
// a field named URL would clash with the route's URL method.
type AddRSS struct {
	gx.Route `POST /sources/add`
	Address  string
}

func (in *AddRSS) Rules() gx.Rules { return gx.Rules{} }
