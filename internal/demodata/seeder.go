package demodata

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"pocket-mvp-backend/internal/security"
)

const (
	DefaultEmail    = "demo@pocket.local"
	DefaultPassword = "PocketDemo123!"
)

type Seeder struct {
	db        *pgxpool.Pool
	protector *security.DataProtector
	email     string
	password  string
}

func New(db *pgxpool.Pool, protector *security.DataProtector, email, password string) *Seeder {
	return &Seeder{db: db, protector: protector, email: email, password: password}
}

func (s *Seeder) Run(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin demo seed: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	ownerID, err := s.seedOwner(ctx, tx)
	if err != nil {
		return err
	}
	if err = resetDemoWorkspace(ctx, tx, ownerID); err != nil {
		return err
	}
	for _, venue := range demoVenues() {
		if err = seedVenue(ctx, tx, ownerID, venue); err != nil {
			return err
		}
		if err = seedCatalog(ctx, tx, venue.ID); err != nil {
			return err
		}
		if err = seedWorkforce(ctx, tx, ownerID, venue.ID); err != nil {
			return err
		}
		if err = seedFloorPlan(ctx, tx, venue.ID); err != nil {
			return err
		}
		if err = seedOrders(ctx, tx, venue.ID); err != nil {
			return err
		}
		if err = seedPayments(ctx, tx, venue.ID); err != nil {
			return err
		}
		if err = seedReviews(ctx, tx, ownerID, venue.ID); err != nil {
			return err
		}
	}
	if err = seedSubscription(ctx, tx, ownerID); err != nil {
		return err
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit demo seed: %w", err)
	}
	return nil
}
