package sources

import (
	"context"

	"github.com/alternayte/drel"
	"github.com/google/uuid"
)

// Subscription selects a catalog source for one reader.
type Subscription struct {
	drel.Model[int64]
	UserID   uuid.UUID `db:"user_id,references=auth_users.id,on_delete=cascade,unique_index=uq_subscriptions"`
	SourceID int64     `db:"source_id,references=sources.id,on_delete=cascade,unique_index=uq_subscriptions"`
}

// SubscribeDefaults subscribes a reader to every enabled source of tier 1
// and 2.
func SubscribeDefaults(ctx context.Context, engine *drel.Engine, userID uuid.UUID) error {
	return engine.WithTx(ctx, func(ctx context.Context) error {
		_, err := drel.MustFromContext(ctx).Exec(ctx, `
			INSERT INTO subscriptions (user_id, source_id)
			SELECT $1, id FROM sources WHERE enabled AND tier <= 2
			ON CONFLICT (user_id, source_id) DO NOTHING`, userID)
		return err
	})
}

// SetSubscribed subscribes or unsubscribes a reader.
func SetSubscribed(ctx context.Context, engine *drel.Engine, userID uuid.UUID, sourceID int64, on bool) error {
	return engine.WithTx(ctx, func(ctx context.Context) error {
		tx := drel.MustFromContext(ctx)
		if on {
			_, err := tx.Exec(ctx, `INSERT INTO subscriptions (user_id, source_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, sourceID)
			return err
		}
		_, err := tx.Exec(ctx, `DELETE FROM subscriptions WHERE user_id = $1 AND source_id = $2`, userID, sourceID)
		return err
	})
}

// Listed is a source as one reader sees it.
type Listed struct {
	ID         int64  `json:"id"`
	Kind       string `json:"kind"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	Tier       int    `json:"tier"`
	Subscribed bool   `json:"subscribed"`
}

// ForReader lists the enabled catalog sources and the reader's own custom
// feeds, with the reader's subscription to each.
func ForReader(ctx context.Context, engine *drel.Engine, userID uuid.UUID) ([]Listed, error) {
	var out []Listed
	err := engine.WithTx(ctx, func(ctx context.Context) error {
		rows, err := drel.MustFromContext(ctx).Query(ctx, `
			SELECT s.id, s.kind::text, s.title, s.url, s.tier,
				EXISTS (SELECT 1 FROM subscriptions u WHERE u.user_id = $1 AND u.source_id = s.id)
			FROM sources s
			WHERE s.enabled AND (s.tier <= 2 OR EXISTS (SELECT 1 FROM subscriptions u WHERE u.user_id = $1 AND u.source_id = s.id))
			ORDER BY s.kind, s.title`, userID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var l Listed
			if err := rows.Scan(&l.ID, &l.Kind, &l.Title, &l.URL, &l.Tier, &l.Subscribed); err != nil {
				return err
			}
			out = append(out, l)
		}
		return rows.Err()
	})
	return out, err
}
