package billing

import (
	"context"

	"pocket-mvp-backend/internal/access"
)

func (s *Service) ListPayments(ctx context.Context, ownerID, venueID string) ([]Payment, error) {
	if err := s.authorizer.Require(ctx, ownerID, venueID, access.ManageVenue); err != nil {
		return nil, err
	}
	return s.repository.ListPayments(ctx, venueID)
}
