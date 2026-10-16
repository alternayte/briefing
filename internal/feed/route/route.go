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

// Story is the full view of one story. Explain carries a notice code from
// the Explain form.
type Story struct {
	gx.Route `GET /s/{ID}`
	ID       int64
	Explain  string `query:"explain"`
}

// Explain asks for an explanation of a story at the reader's level.
type Explain struct {
	gx.Route `POST /s/{StoryID}/explain`
	StoryID  int64
}

func (in *Explain) Rules() gx.Rules { return gx.Rules{} }

// Feedback records what the reader thinks of a story. From names the page to
// return to: "story" or the feed.
type Feedback struct {
	gx.Route `POST /s/{StoryID}/feedback`
	StoryID  int64
	Kind     string
	From     string
}

func (in *Feedback) Rules() gx.Rules {
	return gx.Rules{gx.Field(&in.Kind, gx.Required, gx.OneOf("useful", "knew", "irrelevant", "misleading"))}
}

// Settings is the settings page. Feed carries a notice code from AddRSS.
type Settings struct {
	gx.Route `GET /settings`
	Feed     string `query:"feed"`
	Saved    bool   `query:"saved"`
}

// SetLocale changes the reader's language.
type SetLocale struct {
	gx.Route `POST /settings/locale`
	Locale   string
}

func (in *SetLocale) Rules() gx.Rules { return gx.Rules{} }

// SetBudget changes how many stories a day the feed shows.
type SetBudget struct {
	gx.Route `POST /settings/budget`
	Budget   int
}

func (in *SetBudget) Rules() gx.Rules { return gx.Rules{} }

// InterestsList lists the reader's interests.
type InterestsList struct {
	gx.Route `GET /api/interests`
}

// InterestsAdd adds one interest; Negative marks a topic to leave out.
type InterestsAdd struct {
	gx.Route `POST /settings/interests`
	Text     string
	Negative bool
}

func (in *InterestsAdd) Rules() gx.Rules { return gx.Rules{} }

// InterestsRemove removes one interest.
type InterestsRemove struct {
	gx.Route `POST /settings/interests/{ID}/remove`
	ID       int64
}

func (in *InterestsRemove) Rules() gx.Rules { return gx.Rules{} }
