package appfault

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
)

// MapWriteError converts storage constraint errors into stable application errors.
func MapWriteError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch pgErr.Code {
	case "23505":
		return ErrConflict
	case "23503", "23514":
		return ErrInvalidInput
	default:
		return err
	}
}
