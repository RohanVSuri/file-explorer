package db

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrNotFound      = errors.New("not_found")
	ErrNameConflict  = errors.New("name_conflict")
	ErrCycleDetected = errors.New("cycle_detected")
)

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
