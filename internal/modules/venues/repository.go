package venues

import "context"

type Repository interface {
	List(context.Context, string) ([]Venue, error)
	SearchPublished(context.Context, PublicVenueQuery) ([]PublicVenue, int, error)
	GetPublished(context.Context, string) (PublicVenue, error)
	ListPublishedMenu(context.Context, string) ([]PublicMenuCategory, error)
	ListPublishedReviews(context.Context, string) ([]PublicReview, error)
	IsPublished(context.Context, string) (bool, error)
	Create(context.Context, string, string, Input) (Venue, error)
	Update(context.Context, string, string, Input) (Venue, error)
	Delete(context.Context, string, string) error
}
