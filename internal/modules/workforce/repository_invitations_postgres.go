package workforce

import (
	"bytes"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/security"
)

// The row lock serializes sending, acceptance, role changes and revocation.
func (r *PostgresRepository) IssueInvitation(ctx context.Context, venueID, staffID, hash string) (string, error) {
	var email string
	err := r.db.QueryRow(ctx, `
 UPDATE venue_staff SET invite_token_hash=$3,invite_expires_at=now()+interval '72 hours',invite_sent_at=now()
 WHERE id=$2 AND venue_id=$1 AND status='invited' AND user_id IS NULL AND removed_at IS NULL
 AND (invite_sent_at IS NULL OR invite_sent_at < now()-interval '1 minute')
 AND EXISTS(SELECT 1 FROM venues WHERE id=$1 AND deleted_at IS NULL)
 RETURNING invited_email`, venueID, staffID, hash).Scan(&email)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", appfault.ErrConflict
	}
	return email, err
}
func (r *PostgresRepository) ClearInvitation(ctx context.Context, id, hash string) error {
	_, err := r.db.Exec(ctx, `UPDATE venue_staff SET invite_token_hash=NULL,invite_expires_at=NULL,invite_sent_at=NULL WHERE id=$1 AND invite_token_hash=$2`, id, hash)
	return err
}
func (r *PostgresRepository) ReadInvitation(ctx context.Context, userID, accountEmail string, lookup []byte, hash string, accept bool) (Invitation, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Invitation{}, err
	}
	defer tx.Rollback(ctx)
	var result Invitation
	var id, email string
	var currentLookup []byte
	// Lock the account as well: an email change cannot race with acceptance.
	err = tx.QueryRow(ctx, `SELECT email_lookup FROM users WHERE id=$1 AND status='active' AND deleted_at IS NULL FOR UPDATE`, userID).Scan(&currentLookup)
	if err != nil {
		return result, appfault.ErrForbidden
	}
	if !bytes.Equal(currentLookup, lookup) {
		return result, appfault.ErrForbidden
	}
	err = tx.QueryRow(ctx, `
 SELECT s.id::text,s.invited_email,s.venue_id::text,v.name,s.role,s.invite_expires_at
 FROM venue_staff s JOIN venues v ON v.id=s.venue_id
 WHERE s.invite_token_hash=$1 AND s.status='invited' AND s.user_id IS NULL
 AND s.removed_at IS NULL AND s.invite_expires_at>now() AND v.deleted_at IS NULL
 FOR UPDATE OF s`, hash).Scan(&id, &email, &result.VenueID, &result.VenueName, &result.Role, &result.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, appfault.ErrNotFound
	}
	if err != nil {
		return result, err
	}
	if security.NormalizeEmail(email) != security.NormalizeEmail(accountEmail) {
		return result, appfault.ErrForbidden
	}
	if accept {
		_, err = tx.Exec(ctx, `UPDATE venue_staff SET user_id=$2,status='active',accepted_at=now(),invite_token_hash=NULL,invite_expires_at=NULL WHERE id=$1`, id, userID)
		if err != nil {
			return result, appfault.MapWriteError(err)
		}
	}
	return result, tx.Commit(ctx)
}
