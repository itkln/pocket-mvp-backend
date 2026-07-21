package billing

import "context"

func (s *Service) ListPayments(ctx context.Context, ownerID, venueID string) ([]Payment, error) {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `
		SELECT id::text, provider, status, amount_minor, refunded_minor, currency, paid_at, created_at
		FROM payments
		WHERE venue_id = $1
		ORDER BY created_at DESC
		LIMIT 200`, venueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []Payment{}
	for rows.Next() {
		var payment Payment
		if err := rows.Scan(
			&payment.ID, &payment.Provider, &payment.Status, &payment.AmountMinor,
			&payment.RefundedMinor, &payment.Currency, &payment.PaidAt, &payment.CreatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, payment)
	}
	return result, rows.Err()
}
