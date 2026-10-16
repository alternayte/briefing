// Package route holds the route types of the learning slice.
package route

import "github.com/alternayte/gx"

// Review is the review session. Answered and Choice report the last answer.
type Review struct {
	gx.Route `GET /review`
	Answered int64 `query:"answered"`
	Choice   int   `query:"choice" default:"-1"`
}

// ReviewDue lists the questions due now.
type ReviewDue struct {
	gx.Route `GET /api/review/due`
}

// ReviewAnswer answers one due question with the index of a choice.
type ReviewAnswer struct {
	gx.Route     `POST /review/{ReviewItemID}/answer`
	ReviewItemID int64
	Choice       int
}

func (in *ReviewAnswer) Rules() gx.Rules {
	return gx.Rules{gx.Field(&in.Choice, gx.Min(0), gx.Max(9))}
}

// LearnedPage shows the concepts by state.
type LearnedPage struct {
	gx.Route `GET /learned`
}

// Learned lists the reader's concepts by state.
type Learned struct {
	gx.Route `GET /api/learned`
}
