// Package stories filters and triages items and clusters them into stories.
package stories

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/alternayte/drel"

	"github.com/alternayte/briefing/internal/ingest"
	"github.com/alternayte/briefing/internal/platform"
)

const (
	ReasonEmptyTitle    = "empty_title"
	ReasonTooOld        = "too_old"
	ReasonBlockedDomain = "blocked_domain"
	ReasonDenyPattern   = "deny_pattern"
)

// Rules are the checks that drop an item before any model sees it.
type Rules struct {
	maxAge  time.Duration
	blocked []string
	deny    []*regexp.Regexp
}

func NewRules(t platform.FilterTuning) (*Rules, error) {
	r := &Rules{maxAge: t.MaxItemAge, blocked: t.BlockedDomains}
	for _, p := range t.DenyPatterns {
		re, err := regexp.Compile("(?i)" + p)
		if err != nil {
			return nil, fmt.Errorf("deny pattern %q: %w", p, err)
		}
		r.deny = append(r.deny, re)
	}
	return r, nil
}

// Check returns why the item is dropped, or "" when it passes. An item with
// no publication date counts as new.
func (r *Rules) Check(it *ingest.Item, now time.Time) string {
	title := strings.TrimSpace(it.Title)
	switch {
	case title == "":
		return ReasonEmptyTitle
	case it.PublishedAt != nil && now.Sub(*it.PublishedAt) > r.maxAge:
		return ReasonTooOld
	case slices.Contains(r.blocked, ingest.RegistrableDomain(it.CanonicalURL)):
		return ReasonBlockedDomain
	}
	for _, re := range r.deny {
		if re.MatchString(title) {
			return ReasonDenyPattern
		}
	}
	return ""
}

// Filter applies the rules to new items.
type Filter struct {
	engine *drel.Engine
	rules  *Rules
	now    func() time.Time
}

func NewFilter(engine *drel.Engine, t platform.Tuning, now func() time.Time) (*Filter, error) {
	rules, err := NewRules(t.Filter)
	if err != nil {
		return nil, err
	}
	return &Filter{engine: engine, rules: rules, now: now}, nil
}

// Apply moves each new item among ids to filtered or passed, and returns the
// ids that passed, after it hands them to next in the same transaction. Items
// in any other state stay as they are, so a re-run changes nothing.
func (f *Filter) Apply(ctx context.Context, ids []int64, next platform.Next) ([]int64, error) {
	var passed []int64
	now := f.now()
	err := f.engine.WithTx(ctx, func(ctx context.Context) error {
		repo := drel.NewTxRepository(drel.MustFromContext(ctx), ingest.ItemMeta)
		items, err := repo.Where(drel.And(ingest.Items.ID.In(ids...), ingest.Items.Status.Eq(ingest.ItemNew))).All(ctx)
		if err != nil {
			return err
		}
		for _, it := range items {
			if reason := f.rules.Check(it, now); reason != "" {
				it.Status, it.FilterReason = ingest.ItemFiltered, reason
				continue
			}
			it.Status = ingest.ItemPassed
			passed = append(passed, it.ID())
		}
		if len(passed) == 0 || next == nil {
			return nil
		}
		return next(ctx, passed)
	})
	if err != nil {
		return nil, fmt.Errorf("rule filter: %w", err)
	}
	return passed, nil
}
