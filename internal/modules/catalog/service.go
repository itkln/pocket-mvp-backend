package catalog

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type VenueAuthorizer interface {
	RequireOwner(context.Context, string, string) error
}

type Service struct {
	db         *pgxpool.Pool
	authorizer VenueAuthorizer
}

func NewService(db *pgxpool.Pool, authorizer VenueAuthorizer) *Service {
	return &Service{db: db, authorizer: authorizer}
}
