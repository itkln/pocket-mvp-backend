package venues

import "context"

type Repository interface {
	List(context.Context, string) ([]Venue, error)
	Create(context.Context, string, string, Input) (Venue, error)
	Update(context.Context, string, string, Input) (Venue, error)
	Delete(context.Context, string, string) error
}
