package ingest

import (
	"time"

	"github.com/alternayte/drel"

	"github.com/alternayte/briefing/internal/platform"
)

type ItemStatus string

const (
	// ItemNew waits for the rule filter.
	ItemNew ItemStatus = "new"
	// ItemFiltered failed a rule; it never reaches a model.
	ItemFiltered ItemStatus = "filtered"
	// ItemPassed passed the rules and waits for triage.
	ItemPassed ItemStatus = "passed"
	// ItemRejected failed triage.
	ItemRejected ItemStatus = "rejected"
	// ItemKept passed triage.
	ItemKept ItemStatus = "kept"
)

type TextSource string

const (
	TextPending TextSource = "pending"
	TextNone    TextSource = "none"
	TextExtract TextSource = "extract"
	TextRender  TextSource = "render"
)

// Triage holds the decision-model answers about an item.
type Triage struct {
	Worth        float64   `json:"worth"`
	Sensational  float64   `json:"sensational"`
	Category     string    `json:"category"`
	Significance int       `json:"significance"`
	Learning     int       `json:"learning"`
	Model        string    `json:"model"`
	DecidedAt    time.Time `json:"decided_at"`
}

// Item is one article, whoever linked to it.
type Item struct {
	drel.Model[int64]
	CanonicalURL string          `db:"canonical_url,unique"`
	Title        string          `db:"title"`
	Snippet      string          `db:"snippet"`
	PublishedAt  *time.Time      `db:"published_at"`
	Text         string          `db:"text,default=''"`
	TextLen      int             `db:"text_len,default=0"`
	TextSource   TextSource      `db:"text_source,default=pending"`
	Triage       *Triage         `db:"triage"`
	Embedding    platform.Vector `db:"embedding,type=vector"`
	EmbedModel   string          `db:"embed_model,default=''"`
	Status       ItemStatus      `db:"status,default=new,index=idx_items_status"`
	FilterReason string          `db:"filter_reason,default=''"`
}

// Sighting is one appearance of an item in one source.
type Sighting struct {
	drel.Model[int64]
	SourceID   int64      `db:"source_id,references=sources.id,on_delete=cascade,unique_index=uq_sightings_source_external"`
	ExternalID string     `db:"external_id,unique_index=uq_sightings_source_external"`
	ItemID     int64      `db:"item_id,references=items.id,on_delete=cascade,index=idx_sightings_item"`
	Engagement Engagement `db:"engagement"`
	SeenAt     time.Time  `db:"seen_at"`
}
