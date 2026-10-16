package stories

import (
	"time"

	"github.com/alternayte/drel"

	"github.com/alternayte/briefing/internal/platform"
)

type StoryStatus string

const (
	StoryOpen   StoryStatus = "open"
	StoryClosed StoryStatus = "closed"
)

// Story is a cluster of items that report one event.
type Story struct {
	drel.Model[int64]
	Revision       int             `db:"revision,default=0"`
	Centroid       platform.Vector `db:"centroid,type=vector"`
	OpenedAt       time.Time       `db:"opened_at"`
	LastItemAt     time.Time       `db:"last_item_at,index=idx_stories_open_recent"`
	PublisherCount int             `db:"publisher_count,default=1"`
	Significance   float64         `db:"significance,default=0"`
	Learning       float64         `db:"learning,default=0"`
	Sensational    float64         `db:"sensational,default=0"`
	Category       string          `db:"category"`
	Status         StoryStatus     `db:"status,default=open,index=idx_stories_open_recent"`
}

// StoryItem puts one item in one story.
type StoryItem struct {
	drel.Model[int64]
	StoryID int64 `db:"story_id,references=stories.id,on_delete=cascade,unique_index=uq_story_items"`
	ItemID  int64 `db:"item_id,references=items.id,on_delete=cascade,unique_index=uq_story_items,unique_index=uq_story_items_item"`
}
