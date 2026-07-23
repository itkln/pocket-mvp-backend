package catalog

import "context"

type VenueAuthorizer interface {
	RequireOwner(context.Context, string, string) error
}

type Service struct {
	repository Repository
	authorizer VenueAuthorizer
}

func NewService(repository Repository, authorizer VenueAuthorizer) *Service {
	return &Service{repository: repository, authorizer: authorizer}
}
