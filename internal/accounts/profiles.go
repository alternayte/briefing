package accounts

import (
	"context"
	"fmt"
	"time"

	"github.com/alternayte/drel"
	"github.com/google/uuid"

	"github.com/alternayte/briefing/internal/i18n"
	"github.com/alternayte/briefing/internal/platform"
)

// ProfileStore reads and writes user profiles.
type ProfileStore struct {
	engine        *drel.Engine
	defaultBudget int
}

func NewProfileStore(engine *drel.Engine, t platform.Tuning) *ProfileStore {
	return &ProfileStore{engine: engine, defaultBudget: t.Rank.DefaultDailyFeed}
}

func txProfiles(ctx context.Context) *TxProfileRepository {
	return &TxProfileRepository{TxRepository: drel.NewTxRepository(drel.MustFromContext(ctx), ProfileMeta)}
}

// Ensure returns the profile of a user and creates it on first use. A
// concurrent first request creates nothing twice.
func (s *ProfileStore) Ensure(ctx context.Context, userID uuid.UUID, displayName string, locale i18n.Locale) (*Profile, error) {
	var p *Profile
	err := s.engine.WithTx(ctx, func(ctx context.Context) error {
		repo := txProfiles(ctx)
		_, err := repo.BulkUpsert(ctx, []*Profile{newProfile(userID, displayName, locale, s.defaultBudget)},
			drel.ConflictColumns(Profiles.ID), drel.DoNothing())
		if err != nil {
			return err
		}
		p, err = repo.FindByID(ctx, userID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("ensure profile %s: %w", userID, err)
	}
	return p, nil
}

func (s *ProfileStore) Get(ctx context.Context, userID uuid.UUID) (*Profile, error) {
	var p *Profile
	err := s.engine.WithTx(ctx, func(ctx context.Context) (err error) {
		p, err = txProfiles(ctx).FindByID(ctx, userID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("profile %s: %w", userID, err)
	}
	return p, nil
}

func (s *ProfileStore) SetLocale(ctx context.Context, userID uuid.UUID, l i18n.Locale) error {
	err := s.engine.WithTx(ctx, func(ctx context.Context) error {
		p, err := txProfiles(ctx).FindByID(ctx, userID)
		if err != nil {
			return err
		}
		p.SetLocale(l)
		return nil
	})
	if err != nil {
		return fmt.Errorf("set locale of %s: %w", userID, err)
	}
	return nil
}

// MarkOnboarded records that the user finished onboarding.
func (s *ProfileStore) MarkOnboarded(ctx context.Context, userID uuid.UUID, at time.Time) error {
	err := s.engine.WithTx(ctx, func(ctx context.Context) error {
		p, err := txProfiles(ctx).FindByID(ctx, userID)
		if err != nil {
			return err
		}
		p.OnboardedAt = &at
		return nil
	})
	if err != nil {
		return fmt.Errorf("mark %s onboarded: %w", userID, err)
	}
	return nil
}

// SetDailyBudget sets how many stories a day the feed delivers.
func (s *ProfileStore) SetDailyBudget(ctx context.Context, userID uuid.UUID, budget int) error {
	err := s.engine.WithTx(ctx, func(ctx context.Context) error {
		p, err := txProfiles(ctx).FindByID(ctx, userID)
		if err != nil {
			return err
		}
		p.DailyBudget = budget
		return nil
	})
	if err != nil {
		return fmt.Errorf("set budget of %s: %w", userID, err)
	}
	return nil
}

// SetTimezone sets the reader's IANA zone. An unknown name is an error.
func (s *ProfileStore) SetTimezone(ctx context.Context, userID uuid.UUID, name string) error {
	if _, err := time.LoadLocation(name); err != nil || name == "" {
		return fmt.Errorf("unknown time zone %q", name)
	}
	err := s.engine.WithTx(ctx, func(ctx context.Context) error {
		p, err := txProfiles(ctx).FindByID(ctx, userID)
		if err != nil {
			return err
		}
		p.Timezone = name
		return nil
	})
	if err != nil {
		return fmt.Errorf("set time zone of %s: %w", userID, err)
	}
	return nil
}
