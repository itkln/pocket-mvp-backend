-- V-TBL-05: a guest calls the staff to settle the table bill, and a staff member confirms the
-- in-person payment. The call is not a payment: only the confirmation closes the visit.
ALTER TABLE bills ADD COLUMN payment_requested_at timestamptz;

-- An in-person settlement is a payment with provider 'staff': who confirmed it and how the guest paid.
ALTER TABLE payments
  ADD COLUMN method text CHECK (method IN ('cash', 'card')),
  ADD COLUMN recorded_by_user_id uuid REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX bills_payment_requested_idx ON bills (venue_id, payment_requested_at)
  WHERE status IN ('open', 'payment_pending') AND payment_requested_at IS NOT NULL;
