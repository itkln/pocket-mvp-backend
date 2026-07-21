package access

import (
	"context"
	"testing"
)

type venueAuthorizerContract interface {
	RequireOwner(context.Context, string, string) error
}

func TestVenueAuthorizerImplementsModuleContract(t *testing.T) {
	var _ venueAuthorizerContract = (*VenueAuthorizer)(nil)
}
