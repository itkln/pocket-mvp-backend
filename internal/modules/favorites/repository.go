package favorites

import "context"

// Repository owns only favorite IDs. Venue and identity data belong to their
// respective contexts and are never copied into this table.
type Repository interface {
	ListIDs(context.Context, string) ([]string, error)
	Add(context.Context, string, string) error
	Remove(context.Context, string, string) error
}

// VenueAvailability is a replaceable port: a future remote catalog adapter can
// replace the in-process venue service without changing favorites use cases.
type VenueAvailability interface {
	IsPublished(context.Context, string) (bool, error)
}
