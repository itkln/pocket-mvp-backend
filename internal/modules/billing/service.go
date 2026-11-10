package billing

import (
	"context"

	"pocket-mvp-backend/internal/access"
)

type VenueAuthorizer interface {
	Require(context.Context, string, string, access.Capability) error
}

type Service struct {
	repository Repository
	authorizer VenueAuthorizer
}

func NewService(repository Repository, authorizer VenueAuthorizer) *Service {
	return &Service{repository: repository, authorizer: authorizer}
}
