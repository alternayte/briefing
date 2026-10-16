package feed

import (
	"context"
	"fmt"
	"strings"

	"github.com/alternayte/drel"
	"github.com/google/uuid"

	"github.com/alternayte/briefing/internal/embed"
)

const (
	maxTopics     = 20
	maxTopicChars = 80
)

// parseTopics reads one topic per line, trimmed, without blanks or repeats.
func parseTopics(text string) []string {
	var out []string
	seen := map[string]bool{}
	for line := range strings.SplitSeq(text, "\n") {
		t := strings.Join(strings.Fields(line), " ")
		if k := strings.ToLower(t); t != "" && !seen[k] {
			seen[k] = true
			out = append(out, t)
		}
	}
	return out
}

func topicsValid(topics []string) bool {
	if len(topics) > maxTopics {
		return false
	}
	for _, t := range topics {
		if len([]rune(t)) > maxTopicChars {
			return false
		}
	}
	return true
}

// replaceInterests sets the reader's interests. The positive ones get an
// embedding for the prefilter; the negative ones are only read by the
// relevance decision.
func replaceInterests(ctx context.Context, engine *drel.Engine, e *embed.Embedder, userID uuid.UUID, positive, negative []string) error {
	vectors, err := e.Embed(ctx, "interest_embed", positive)
	if err != nil {
		return err
	}
	return engine.WithTx(ctx, func(ctx context.Context) error {
		tx := drel.MustFromContext(ctx)
		if _, err := tx.Exec(ctx, `DELETE FROM interests WHERE user_id = $1`, userID); err != nil {
			return err
		}
		for i, text := range positive {
			v, err := vectors[i].Value()
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO interests (user_id, text, embedding, embed_model) VALUES ($1, $2, $3::vector, $4)`,
				userID, text, v, e.Model()); err != nil {
				return fmt.Errorf("store interest: %w", err)
			}
		}
		for _, text := range negative {
			if _, err := tx.Exec(ctx, `INSERT INTO interests (user_id, text, negative) VALUES ($1, $2, true)`, userID, text); err != nil {
				return fmt.Errorf("store interest: %w", err)
			}
		}
		return nil
	})
}

// interestsOf returns the reader's interests as two newline-separated texts.
func interestsOf(ctx context.Context, tx *drel.Tx, userID uuid.UUID) (positive, negative string, err error) {
	rows, err := tx.Query(ctx, `SELECT text, negative FROM interests WHERE user_id = $1 ORDER BY id`, userID)
	if err != nil {
		return "", "", err
	}
	defer rows.Close()
	var pos, neg []string
	for rows.Next() {
		var text string
		var isNeg bool
		if err := rows.Scan(&text, &isNeg); err != nil {
			return "", "", err
		}
		if isNeg {
			neg = append(neg, text)
		} else {
			pos = append(pos, text)
		}
	}
	return strings.Join(pos, "\n"), strings.Join(neg, "\n"), rows.Err()
}
