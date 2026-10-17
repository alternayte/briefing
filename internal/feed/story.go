package feed

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/alternayte/drel"
	"github.com/alternayte/gx"
	"github.com/google/uuid"

	"github.com/alternayte/briefing/internal/accounts"
	"github.com/alternayte/briefing/internal/feed/route"
	"github.com/alternayte/briefing/internal/i18n"
	"github.com/alternayte/briefing/internal/ingest"
	"github.com/alternayte/briefing/internal/learning"
	"github.com/alternayte/briefing/internal/translate"
)

// maxShownText caps the extracted text the page shows for one source.
const maxShownText = 20000

type ClaimView struct {
	Text string
	Refs []SourceRef
}

type SourceRef struct {
	N   int
	URL gx.URL
}

type ConceptView struct {
	ID         int64
	Name       string
	Definition string
	State      string
	Mastered   bool
}

type SourceView struct {
	ItemID     int64
	N          int
	Domain     string
	Title      string
	URL        gx.URL
	Paragraphs []string
	// Translated is the cached machine translation; Translate offers one.
	Translated   []string
	Translate    TranslateArticleButtonProps
	CanTranslate bool
}

// StoryPage is the full story view.
var StoryPage = gx.Page(func(c *gx.Ctx, in route.Read) (StoryViewProps, error) {
	return storyView(c.R.Context(), in, true)
}, StoryView)

// storyView builds the story page of the current reader. With open it also
// records that they opened the story.
func storyView(ctx context.Context, in route.Read, open bool) (StoryViewProps, error) {
	w, profile := webFrom(ctx), accounts.CurrentProfile(ctx)
	msg := i18n.For(i18n.FromContext(ctx))
	p := StoryViewProps{Msg: msg, StoryID: in.ID, Budget: profile.DailyBudget, Zone: profile.Location().String()}
	err := w.engine.WithTx(ctx, func(ctx context.Context) error {
		tx := drel.MustFromContext(ctx)
		found, err := loadStory(ctx, tx, in.ID, profile.ID(), w.viewOf(ctx, profile), &p)
		if err != nil || !found || !open {
			return err
		}
		return markOpened(ctx, tx, profile.ID(), in.ID)
	})
	if err == nil && p.Headline == "" && len(p.Sources) == 0 {
		return p, gx.NotFound()
	}
	if err != nil || p.LinkCard {
		return p, err
	}
	locale := i18n.FromContext(ctx)
	text, err := w.explainer.Cached(ctx, profile.ID(), in.ID, locale)
	p.Explanation = paragraphs(strings.ReplaceAll(text, "\n", "\n\n"))
	p.ExplainForm = Explain.Props(&route.Explain{StoryID: in.ID})
	p.ExplainForm.Label = msg.ExplainButton
	switch in.Explain {
	case "cap":
		p.ExplainNotice = msg.ExplainCap
	case "limit":
		p.ExplainNotice = msg.ExplainLimit
	}
	p.ArticleNotice = map[string]string{"cap": msg.ArticleCap, "limit": msg.ArticleLimit}[in.Article]
	return p, err
}

func loadStory(ctx context.Context, tx *drel.Tx, storyID int64, userID uuid.UUID, v view, p *StoryViewProps) (bool, error) {
	locale := v.locale
	msg := i18n.For(locale)
	var revision, publishers int
	var shown *int
	var openedAt, lastItemAt time.Time
	err := tx.QueryRow(ctx, `
		SELECT s.revision, s.publisher_count, (SELECT shown_revision FROM feed_entries WHERE user_id = $2 AND story_id = s.id),
			s.opened_at, s.last_item_at
		FROM stories s WHERE s.id = $1`, storyID, userID).Scan(&revision, &publishers, &shown, &openedAt, &lastItemAt)
	if isNoRows(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	p.Updated = shown != nil && *shown > 0 && *shown < revision
	p.Dates = fmt.Sprintf(msg.StoryDates, i18n.DateTimeIn(locale, openedAt, v.loc), i18n.DateTimeIn(locale, lastItemAt, v.loc))

	rows, err := tx.Query(ctx, `
		SELECT i.id, i.canonical_url, i.title, i.text FROM story_items si JOIN items i ON i.id = si.item_id
		WHERE si.story_id = $1 ORDER BY i.text_len DESC, i.id`, storyID)
	if err != nil {
		return false, err
	}
	refOf := map[int64]SourceRef{}
	for rows.Next() {
		var id int64
		var u, title, text string
		if err := rows.Scan(&id, &u, &title, &text); err != nil {
			rows.Close()
			return false, err
		}
		n := len(p.Sources) + 1
		refOf[id] = SourceRef{N: n, URL: gx.URL(u)}
		p.Sources = append(p.Sources, SourceView{ItemID: id, N: n, Domain: ingest.RegistrableDomain(u), Title: title, URL: gx.URL(u), Paragraphs: paragraphs(text)})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return false, err
	}

	var claims []byte
	var confidence string
	err = tx.QueryRow(ctx, `SELECT headline, summary, why_it_matters, what_is_new, claims, confidence::text
		FROM summaries WHERE story_id = $1 ORDER BY revision DESC LIMIT 1`, storyID).
		Scan(&p.Headline, &p.Summary, &p.Why, &p.New, &claims, &confidence)
	switch {
	case isNoRows(err):
		p.LinkCard = true
		if len(p.Sources) > 0 {
			p.Headline = p.Sources[0].Title
		}
	case err != nil:
		return false, err
	default:
		var cs []struct {
			Text    string  `json:"text"`
			ItemIDs []int64 `json:"item_ids"`
		}
		if err := json.Unmarshal(claims, &cs); err != nil {
			return false, fmt.Errorf("claims of story %d: %w", storyID, err)
		}
		for _, c := range cs {
			cv := ClaimView{Text: c.Text}
			for _, id := range c.ItemIDs {
				if r, ok := refOf[id]; ok {
					cv.Refs = append(cv.Refs, r)
				}
			}
			p.Claims = append(p.Claims, cv)
		}
		if publishers > 1 {
			p.Meta = append(p.Meta, fmt.Sprintf(msg.FeedPublishers, publishers))
		}
		if word := map[string]string{"high": msg.ConfidenceHigh, "medium": msg.ConfidenceMedium, "low": msg.ConfidenceLow}[confidence]; word != "" {
			p.Meta = append(p.Meta, fmt.Sprintf(msg.FeedConfidence, word))
		}
	}

	states, err := learning.ConceptStates(ctx, tx, userID)
	if err != nil {
		return false, err
	}
	crow, err := tx.Query(ctx, `SELECT c.id, c.name, c.definition FROM story_concepts sc JOIN concepts c ON c.id = sc.concept_id
		WHERE sc.story_id = $1 ORDER BY c.name`, storyID)
	if err != nil {
		return false, err
	}
	defer crow.Close()
	for crow.Next() {
		var id int64
		var cv ConceptView
		if err := crow.Scan(&id, &cv.Name, &cv.Definition); err != nil {
			return false, err
		}
		cv.ID = id
		cv.State, cv.Mastered = conceptLabel(states[id], msg), states[id] == learning.StateMastered
		p.Concepts = append(p.Concepts, cv)
	}
	if err := crow.Err(); err != nil {
		return false, err
	}
	given, err := givenFeedback(ctx, tx, userID, storyID)
	if err != nil {
		return false, err
	}
	p.Feedback = feedbackButtons(storyID, "story", given, feedbackLabels(msg))
	p.Flagged = slices.Contains(given, string(learning.EventMisleading))
	return true, translateStory(ctx, tx, storyID, locale, msg, p)
}

// translateStory puts the reader's language into the view: the cached
// translation of the entry and of each source, or a marker and the forms
// that ask for them.
func translateStory(ctx context.Context, tx *drel.Tx, storyID int64, l i18n.Locale, msg i18n.Messages, p *StoryViewProps) error {
	if l == i18n.Locales[0] {
		return nil
	}
	for i := range p.Sources {
		s := &p.Sources[i]
		paras, err := translate.Article(ctx, tx, s.ItemID, l)
		if err != nil {
			return err
		}
		s.Translated = paras
		if len(paras) == 0 && len(s.Paragraphs) > 0 {
			s.CanTranslate = true
			s.Translate = TranslateArticle.Props(&route.TranslateArticle{StoryID: storyID, ItemID: s.ItemID})
			s.Translate.Label = msg.ArticleTranslate
		}
	}
	if p.LinkCard {
		return nil
	}
	f, ok, err := translate.Story(ctx, tx, storyID, l)
	if err != nil || !ok {
		p.Translating = err == nil
		return err
	}
	p.Headline, p.Summary, p.Why, p.New = f.Headline, f.Summary, f.Why, f.New
	for i := range p.Claims {
		if i < len(f.Claims) {
			p.Claims[i].Text = f.Claims[i]
		}
	}
	byID := map[int64]translate.ConceptText{}
	for _, c := range f.Concepts {
		byID[c.ID] = c
	}
	for i := range p.Concepts {
		if c, ok := byID[p.Concepts[i].ID]; ok {
			p.Concepts[i].Name, p.Concepts[i].Definition = c.Name, c.Definition
		}
	}
	return nil
}

func conceptLabel(s learning.ConceptState, msg i18n.Messages) string {
	switch s {
	case learning.StateMastered:
		return msg.ConceptMastered
	case learning.StateFamiliar:
		return msg.ConceptFamiliar
	case learning.StateMet:
		return msg.ConceptMet
	}
	return msg.ConceptNew
}

// markOpened records the first opening of a story and marks its feed entry
// seen at the current revision. A done entry stays done; a done entry back
// on Today for a new revision goes off it again, as read.
func markOpened(ctx context.Context, tx *drel.Tx, userID uuid.UUID, storyID int64) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO learning_events (user_id, story_id, kind, payload)
		SELECT $1, $2, 'opened', '{}'
		WHERE NOT EXISTS (SELECT 1 FROM learning_events WHERE user_id = $1 AND story_id = $2 AND kind = 'opened')`, userID, storyID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		UPDATE feed_entries f SET state = CASE WHEN f.state IN ('candidate', 'done') THEN f.state ELSE 'seen' END,
			shown_revision = s.revision, updated_at = now()
		FROM stories s WHERE s.id = f.story_id AND f.user_id = $1 AND f.story_id = $2`, userID, storyID)
	return err
}

// paragraphs splits extracted text into paragraphs, cut at maxShownText.
func paragraphs(text string) []string {
	if r := []rune(text); len(r) > maxShownText {
		text = string(r[:maxShownText]) + " […]"
	}
	var out []string
	for block := range strings.SplitSeq(text, "\n\n") {
		if p := strings.Join(strings.Fields(block), " "); p != "" {
			out = append(out, p)
		}
	}
	return out
}
