package billing

import "context"

func (s *Service) ListPayments(ctx context.Context, ownerID, venueID string) ([]Payment, error) {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return nil, err
	}
	return s.repository.ListPayments(ctx, venueID)
}
