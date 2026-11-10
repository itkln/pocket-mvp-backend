package identity

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// userColumns are the columns scanUser reads, in its order. Personal fields stay encrypted
// until the service decrypts them.
const userColumns = `id::text, email, first_name, last_name, phone, account_role, avatar_updated_at`

// scanUser reads one active user; a missing row means the session or account is no longer valid.
func scanUser(row pgx.Row, action string) (encryptedUser, error) {
	var record encryptedUser
	err := row.Scan(
		&record.id, &record.email, &record.firstName, &record.lastName,
		&record.phone, &record.role, &record.avatarUpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return encryptedUser{}, ErrUnauthorized
	}
	if err != nil {
		return encryptedUser{}, fmt.Errorf("%s: %w", action, err)
	}
	return record, nil
}
