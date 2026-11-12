package seating

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/money"
	"pocket-mvp-backend/internal/tablesync"
)

type PostgresRepository struct{ db *pgxpool.Pool }

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository { return &PostgresRepository{db: db} }

// A party that was never released stops occupying its table a day later.
const seatedCondition = `r.status = 'seated' AND r.starts_at > now() - interval '24 hours'`

func (r *PostgresRepository) Floor(ctx context.Context, venueID string) (Floor, error) {
	// One snapshot, so a table and the booking that follows it never disagree.
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Floor{}, err
	}
	defer tx.Rollback(ctx)
	floor := Floor{Tables: []Table{}, Bookings: []Booking{}}
	var dayEnd time.Time
	err = tx.QueryRow(ctx, `
		SELECT now(), timezone, currency,
		       (date_trunc('day', now() AT TIME ZONE timezone) + interval '1 day') AT TIME ZONE timezone
		FROM venues WHERE id = $1 AND deleted_at IS NULL`, venueID).
		Scan(&floor.Now, &floor.Timezone, &floor.Currency, &dayEnd)
	if errors.Is(err, pgx.ErrNoRows) {
		return Floor{}, appfault.ErrNotFound
	}
	if err != nil {
		return Floor{}, err
	}
	rows, err := tx.Query(ctx, `
		SELECT t.id::text, t.identifier, t.seats,
		       party.id, party.reservation_code, party.guest_name, party.guest_count, party.seated_at, party.walk_in,
		       bill.id, bill.opened_at, bill.total_minor
		FROM venue_tables t
		LEFT JOIN LATERAL (
		    SELECT r.id::text AS id, r.reservation_code, r.guest_name, r.guest_count,
		           COALESCE(r.seated_at, r.starts_at) AS seated_at, r.walk_in
		    FROM reservations r
		    WHERE r.venue_table_id = t.id AND `+seatedCondition+`
		    ORDER BY r.seated_at DESC NULLS LAST, r.id LIMIT 1
		) party ON true
		LEFT JOIN LATERAL (
		    SELECT b.id::text AS id, b.opened_at, COALESCE((
		        SELECT sum(o.total_minor) FROM orders o
		        WHERE o.bill_id = b.id AND o.status NOT IN ('draft', 'cancelled')
		    ), 0) AS total_minor
		    FROM bills b
		    WHERE b.venue_table_id = t.id AND b.status IN ('open', 'payment_pending')
		    LIMIT 1
		) bill ON true
		WHERE t.venue_id = $1 AND t.deleted_at IS NULL AND t.status <> 'disabled'
		ORDER BY t.identifier, t.id`, venueID)
	if err != nil {
		return Floor{}, err
	}
	for rows.Next() {
		var table Table
		var partyID, code, guestName, billID *string
		var guestCount *int
		var seatedAt, openedAt *time.Time
		var walkIn *bool
		var billTotal *int64
		if err = rows.Scan(&table.ID, &table.Identifier, &table.Seats, &partyID, &code, &guestName,
			&guestCount, &seatedAt, &walkIn, &billID, &openedAt, &billTotal); err != nil {
			rows.Close()
			return Floor{}, err
		}
		if partyID != nil {
			table.Party = &Party{ReservationID: *partyID, Code: *code, GuestName: *guestName,
				GuestCount: *guestCount, SeatedAt: *seatedAt, WalkIn: *walkIn}
		}
		if billID != nil {
			table.Bill = &Bill{ID: *billID, OpenedAt: *openedAt, TotalMinor: money.Minor(*billTotal)}
		}
		floor.Tables = append(floor.Tables, table)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return Floor{}, err
	}
	rows, err = tx.Query(ctx, `
		SELECT r.id::text, r.reservation_code, t.identifier, r.guest_name, COALESCE(r.guest_phone, ''),
		       r.guest_count, r.starts_at, r.ends_at, COALESCE(r.notes, ''),
		       EXISTS (SELECT 1 FROM orders o WHERE o.reservation_id = r.id
		               AND o.channel = 'preorder' AND o.status <> 'cancelled')
		FROM reservations r
		JOIN venue_tables t ON t.id = r.venue_table_id
		WHERE r.venue_id = $1 AND r.status = 'confirmed' AND r.ends_at > now() AND r.starts_at < $2
		ORDER BY r.starts_at, r.id`, venueID, dayEnd)
	if err != nil {
		return Floor{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var booking Booking
		if err = rows.Scan(&booking.ID, &booking.Code, &booking.TableIdentifier, &booking.GuestName,
			&booking.GuestPhone, &booking.GuestCount, &booking.StartsAt, &booking.EndsAt,
			&booking.Notes, &booking.HasPreorder); err != nil {
			return Floor{}, err
		}
		floor.Bookings = append(floor.Bookings, booking)
	}
	if err = rows.Err(); err != nil {
		return Floor{}, err
	}
	resolveStates(&floor)
	return floor, nil
}

// lockTable makes sure the plan's table has a row, locks it and returns its ID.
func lockTable(ctx context.Context, tx pgx.Tx, venueID, identifier string) (string, error) {
	if err := tablesync.Table(ctx, tx, venueID, identifier); err != nil {
		return "", err
	}
	var id string
	err := tx.QueryRow(ctx, `
		SELECT id::text FROM venue_tables
		WHERE venue_id = $1 AND identifier = $2 AND deleted_at IS NULL AND status <> 'disabled'
		FOR UPDATE`, venueID, identifier).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", appfault.ErrNotFound
	}
	return id, err
}

// requireNoParty fails when someone other than the given reservation sits at the locked table.
func requireNoParty(ctx context.Context, tx pgx.Tx, tableID, exceptReservationID string) error {
	var occupied bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM reservations r
		    WHERE r.venue_table_id = $1 AND `+seatedCondition+`
		      AND r.id <> COALESCE(NULLIF($2, '')::uuid, '00000000-0000-0000-0000-000000000000'::uuid))`,
		tableID, exceptReservationID).Scan(&occupied)
	if err != nil {
		return err
	}
	if occupied {
		return ErrTableOccupied
	}
	return nil
}

func slotTaken(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23P01"
}

func (r *PostgresRepository) SeatReservation(ctx context.Context, venueID, reservationID, tableIdentifier string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// The booked table's identifier first: tables are always locked before reservations.
	var bookedIdentifier string
	err = tx.QueryRow(ctx, `
		SELECT t.identifier FROM reservations r JOIN venue_tables t ON t.id = r.venue_table_id
		WHERE r.venue_id = $1 AND r.id = $2`, venueID, reservationID).Scan(&bookedIdentifier)
	if errors.Is(err, pgx.ErrNoRows) {
		return appfault.ErrNotFound
	}
	if err != nil {
		return err
	}
	if tableIdentifier == "" {
		tableIdentifier = bookedIdentifier
	}
	var tableID string
	if tableIdentifier == bookedIdentifier {
		// The booked table may have left the plan since; the booking still holds its row.
		err = tx.QueryRow(ctx, `SELECT id::text FROM venue_tables
			WHERE venue_id = $1 AND identifier = $2 FOR UPDATE`, venueID, tableIdentifier).Scan(&tableID)
	} else {
		tableID, err = lockTable(ctx, tx, venueID, tableIdentifier)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return appfault.ErrNotFound
	}
	if err != nil {
		return err
	}
	var status, currentTableID string
	err = tx.QueryRow(ctx, `SELECT status, venue_table_id::text FROM reservations
		WHERE venue_id = $1 AND id = $2 FOR UPDATE`, venueID, reservationID).Scan(&status, &currentTableID)
	if errors.Is(err, pgx.ErrNoRows) {
		return appfault.ErrNotFound
	}
	if err != nil {
		return err
	}
	if status == "seated" && currentTableID == tableID {
		return nil
	}
	if status != "confirmed" {
		return ErrNotSeatable
	}
	if err = requireNoParty(ctx, tx, tableID, reservationID); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE reservations
		SET status = 'seated', seated_at = now(), venue_table_id = $2 WHERE id = $1`, reservationID, tableID)
	if slotTaken(err) {
		return ErrTableReserved
	}
	if err != nil {
		return appfault.MapWriteError(err)
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) SeatWalkIn(ctx context.Context, venueID string, input WalkIn) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tableID, err := lockTable(ctx, tx, venueID, input.TableIdentifier)
	if err != nil {
		return err
	}
	if err = requireNoParty(ctx, tx, tableID, ""); err != nil {
		return err
	}
	// The party stays the usual visit, or until the table's next booking if that comes sooner.
	var now time.Time
	var nextStart *time.Time
	err = tx.QueryRow(ctx, `
		SELECT now(), (
		    SELECT min(r.starts_at) FROM reservations r
		    WHERE r.venue_table_id = $1 AND r.ends_at > now()
		      AND (r.status = 'confirmed' OR
		           (r.status = 'pending' AND (r.hold_expires_at IS NULL OR r.hold_expires_at > now())))
		)`, tableID).Scan(&now, &nextStart)
	if err != nil {
		return err
	}
	endsAt := now.Add(visit)
	if nextStart != nil && nextStart.Before(endsAt) {
		endsAt = *nextStart
	}
	if endsAt.Before(now.Add(minimumVisit)) {
		return ErrTableReserved
	}
	_, err = tx.Exec(ctx, `WITH new_id AS (SELECT gen_random_uuid() AS id)
		INSERT INTO reservations (id, venue_id, venue_table_id, reservation_code, guest_name,
			guest_count, starts_at, ends_at, status, seated_at, walk_in)
		SELECT id, $1, $2, upper(substr(replace(id::text, '-', ''), 1, 10)), $3, $4, $5, $6,
			'seated', $5, true FROM new_id`,
		venueID, tableID, input.GuestName, input.Guests, now, endsAt)
	if slotTaken(err) {
		return ErrTableReserved
	}
	if err != nil {
		return appfault.MapWriteError(err)
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) Release(ctx context.Context, venueID, tableIdentifier string) error {
	var known bool
	err := r.db.QueryRow(ctx, `
		WITH target AS (
		    SELECT id FROM venue_tables WHERE venue_id = $1 AND identifier = $2 AND deleted_at IS NULL
		), released AS (
		    UPDATE reservations r SET status = 'completed'
		    WHERE r.venue_table_id IN (SELECT id FROM target) AND r.status = 'seated'
		)
		SELECT EXISTS (SELECT 1 FROM target)`, venueID, tableIdentifier).Scan(&known)
	if err != nil {
		return err
	}
	if !known {
		return appfault.ErrNotFound
	}
	return nil
}
