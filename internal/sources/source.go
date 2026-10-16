// Package sources holds the source catalog, subscriptions and poll
// scheduling.
package sources

import (
	"time"

	"github.com/alternayte/drel"
)

type SourceKind string

const (
	KindRSS     SourceKind = "rss"
	KindHN      SourceKind = "hn"
	KindBluesky SourceKind = "bluesky"
)

// Source is one feed the app polls. Every source is global; users subscribe
// to it. URL holds the author handle for a Bluesky source.
type Source struct {
	drel.Model[int64]
	Kind                SourceKind `db:"kind,unique_index=uq_sources_kind_url"`
	URL                 string     `db:"url,unique_index=uq_sources_kind_url"`
	Title               string     `db:"title"`
	Tier                int        `db:"tier"`
	Language            string     `db:"language"`
	Enabled             bool       `db:"enabled,default=true"`
	PollIntervalSeconds int        `db:"poll_interval_seconds"`
	ETag                string     `db:"etag,default=''"`
	LastModified        string     `db:"last_modified,default=''"`
	Cursor              string     `db:"cursor,default=''"`
	LastOKAt            *time.Time `db:"last_ok_at"`
	LastError           string     `db:"last_error,default=''"`
}

func (s *Source) PollInterval() time.Duration {
	return time.Duration(s.PollIntervalSeconds) * time.Second
}
