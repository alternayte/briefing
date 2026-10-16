// Package route holds the route types of the feed slice.
package route

import "github.com/alternayte/gx"

// Home is the feed.
type Home struct {
	gx.Route `GET /{$}`
}

// Fresh reports stories delivered after Since, a Unix time.
type Fresh struct {
	gx.Route `GET /feed/fresh`
	Since    int64 `query:"since"`
}

// Onboarding asks for the locale and the interests.
type Onboarding struct {
	gx.Route `GET /onboarding`
	Error    string `query:"error"`
}

// SaveInterests stores the locale and the interests. The handler validates,
// so a failed submit redirects with an error code.
type SaveInterests struct {
	gx.Route  `POST /onboarding`
	Locale    string
	Interests string
	Avoid     string
}

func (in *SaveInterests) Rules() gx.Rules { return gx.Rules{} }

// Calibrate shows recent stories to rate.
type Calibrate struct {
	gx.Route `GET /onboarding/calibrate`
}

// Rate records the reader's rating of one calibration story.
type Rate struct {
	gx.Route `POST /onboarding/calibrate/{StoryID}`
	StoryID  int64
	Rating   string
}

func (in *Rate) Rules() gx.Rules { return gx.Rules{} }

// Finish ends onboarding and ranks the first feed.
type Finish struct {
	gx.Route `POST /onboarding/finish`
}

func (in *Finish) Rules() gx.Rules { return gx.Rules{} }

// Story is the full view of one story.
type Story struct {
	gx.Route `GET /s/{ID}`
	ID       int64
}
