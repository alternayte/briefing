package learning

import (
	"context"
	"errors"

	"github.com/alternayte/drel"
	"github.com/alternayte/gx"
	"github.com/google/uuid"

	"github.com/alternayte/briefing/internal/accounts"
	"github.com/alternayte/briefing/internal/i18n"
	"github.com/alternayte/briefing/internal/learning/route"
	"github.com/alternayte/briefing/internal/translate"
)

// translateDue puts each due question in the reader's language, from the
// translation of its story.
func translateDue(ctx context.Context, tx *drel.Tx, items []DueItem, l i18n.Locale) error {
	if l == i18n.Locales[0] {
		return nil
	}
	for i := range items {
		f, ok, err := translate.Story(ctx, tx, items[i].StoryID, l)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		items[i].Headline = f.Headline
		for _, r := range f.Review {
			if r.ID == items[i].ID {
				items[i].Question, items[i].Choices, items[i].explanation = r.Question, r.Choices, r.Explanation
			}
		}
	}
	return nil
}

// QuestionView is one due question with its answer forms.
type QuestionView struct {
	DueItem
	Buttons []AnswerButtonProps
}

// ReviewPage is the review session.
var ReviewPage = gx.Page(func(c *gx.Ctx, in route.Review) (ReviewViewProps, error) {
	ctx := c.R.Context()
	w, profile := webFrom(ctx), accounts.CurrentProfile(ctx)
	locale := i18n.FromContext(ctx)
	p := ReviewViewProps{Msg: i18n.For(locale)}
	err := w.engine.WithTx(ctx, func(ctx context.Context) error {
		tx := drel.MustFromContext(ctx)
		if in.Answered != 0 {
			if err := lastAnswer(ctx, tx, profile.ID(), in, locale, &p); err != nil {
				return err
			}
		}
		items, err := Due(ctx, tx, profile.ID(), w.now(), profile.Location(), w.tuning.Review)
		if err != nil {
			return err
		}
		if err := translateDue(ctx, tx, items, locale); err != nil {
			return err
		}
		for _, it := range items {
			q := QuestionView{DueItem: it}
			for i, choice := range it.Choices {
				b := ReviewAnswer.Props(&route.ReviewAnswer{ReviewItemID: it.ID, Choice: i})
				b.Label, b.Choice = choice, i
				q.Buttons = append(q.Buttons, b)
			}
			p.Questions = append(p.Questions, q)
		}
		return nil
	})
	return p, err
}, ReviewView)

// lastAnswer fills the outcome of the answer the reader just gave.
func lastAnswer(ctx context.Context, tx *drel.Tx, userID uuid.UUID, in route.Review, l i18n.Locale, p *ReviewViewProps) error {
	var storyID int64
	var answer, interval int
	var explanation string
	err := tx.QueryRow(ctx, `SELECT r.story_id, r.answer_index, r.explanation, s.interval_days
		FROM review_items r JOIN review_schedules s ON s.review_item_id = r.id AND s.user_id = $1 WHERE r.id = $2`,
		userID, in.Answered).Scan(&storyID, &answer, &explanation, &interval)
	if isNoRows(err) {
		return nil
	}
	if err != nil {
		return err
	}
	items := []DueItem{{ID: in.Answered, StoryID: storyID, explanation: explanation}}
	if err := translateDue(ctx, tx, items, l); err != nil {
		return err
	}
	p.Answered, p.Correct, p.Explanation = true, in.Choice == answer, items[0].explanation
	p.NextIn = i18n.Count(l, interval, p.Msg.ReviewNextInOne, p.Msg.ReviewNextIn)
	return nil
}

// Lists the review questions the reader should answer now, each with its id,
// the story it comes from, the question and the choices. Use it to run a
// review session; answer each with learning_review_answer.
var ReviewDueAction = gx.Action(func(c *gx.Ctx, _ route.ReviewDue) error {
	ctx := c.R.Context()
	w, profile := webFrom(ctx), accounts.CurrentProfile(ctx)
	return w.engine.WithTx(ctx, func(ctx context.Context) error {
		tx := drel.MustFromContext(ctx)
		items, err := Due(ctx, tx, profile.ID(), w.now(), profile.Location(), w.tuning.Review)
		if err == nil {
			err = translateDue(ctx, tx, items, i18n.FromContext(ctx))
		}
		if items == nil {
			items = []DueItem{}
		}
		gx.ToolResult(c, items)
		return err
	})
}).Tool()

// Answers one due review question with the index of the chosen answer,
// counting from 0. Returns whether it was correct, the correct index, an
// explanation and the days until the question comes back. Tell the reader the
// explanation, especially after a wrong answer.
var ReviewAnswer = gx.Form(func(c *gx.Ctx, in *route.ReviewAnswer) error {
	ctx := c.R.Context()
	w, profile := webFrom(ctx), accounts.CurrentProfile(ctx)
	a, err := AnswerReview(ctx, w.engine, profile.ID(), in.ReviewItemID, in.Choice, w.now().UTC(), w.tuning.Review)
	if errors.Is(err, ErrNotDue) {
		return gx.NotFound()
	}
	if err != nil {
		return err
	}
	err = w.engine.WithTx(ctx, func(ctx context.Context) error {
		items := []DueItem{{ID: in.ReviewItemID, explanation: a.Explanation}}
		tx := drel.MustFromContext(ctx)
		if err := tx.QueryRow(ctx, `SELECT story_id FROM review_items WHERE id = $1`, in.ReviewItemID).Scan(&items[0].StoryID); err != nil {
			return err
		}
		if err := translateDue(ctx, tx, items, i18n.FromContext(ctx)); err != nil {
			return err
		}
		a.Explanation = items[0].explanation
		return nil
	})
	gx.ToolResult(c, a)
	if err != nil {
		return err
	}
	return c.Redirect(route.Review{Answered: in.ReviewItemID, Choice: in.Choice})
}, AnswerButton).Tool()
