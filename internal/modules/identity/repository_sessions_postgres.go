package identity

// Sessions: resolving a session token to its user and revoking it.

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func (r *PostgresRepository) FindUserBySession(ctx context.Context, tokenHash string) (encryptedUser, error) {
	return scanUser(r.db.QueryRow(ctx, `
		SELECT `+userColumns+`
		FROM users
		WHERE id = (
			SELECT user_id FROM user_sessions
			WHERE refresh_token_hash = $1 AND revoked_at IS NULL AND expires_at > now()
		) AND status = 'active' AND deleted_at IS NULL`, tokenHash), "authenticate session")
}

func (r *PostgresRepository) RevokeSession(ctx context.Context, tokenHash string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE user_sessions
		SET revoked_at = now()
		WHERE refresh_token_hash = $1 AND revoked_at IS NULL`, tokenHash)
	return err
}

func insertSession(ctx context.Context, tx pgx.Tx, session storedSession) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO user_sessions (user_id, refresh_token_hash, user_agent, ip_address, expires_at)
		VALUES ($1, $2, $3, $4, $5)`,
		session.userID, session.tokenHash, session.userAgent, session.ipAddress, session.expiresAt,
	)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}
