package tablesync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"pocket-mvp-backend/internal/appfault"
)

// ErrTableInUse: the saved plan drops a table that guests still need — an upcoming booking, a
// seated party, an open bill or an order in progress. The plan is not saved.
var ErrTableInUse = errors.New("table_in_use")

const (
	maxIdentifierLength = 32
	maxSeats            = 50
)

type PlanFloor struct {
	Name   string      `json:"name"`
	Tables []PlanTable `json:"tables"`
}

type PlanTable struct {
	ID    string  `json:"id"`
	Seats int     `json:"seats"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Shape string  `json:"shape"`
}

// dbShape maps the editor's table shape to venue_tables.shape.
func (t PlanTable) dbShape() string {
	switch t.Shape {
	case "round", "square":
		return t.Shape
	default:
		return "rectangle"
	}
}

// ParsePlan validates the editor's floor plan: an array of floors whose tables have unique,
// non-empty numbers and a sensible seat count.
func ParsePlan(raw json.RawMessage) ([]PlanFloor, error) {
	var floors []PlanFloor
	if err := json.Unmarshal(raw, &floors); err != nil || floors == nil {
		return nil, appfault.ErrInvalidInput
	}
	seen := map[string]bool{}
	for i := range floors {
		floors[i].Name = strings.TrimSpace(floors[i].Name)
		if floors[i].Name == "" {
			floors[i].Name = fmt.Sprintf("Зал %d", i+1)
		}
		for _, table := range floors[i].Tables {
			if table.ID == "" || table.ID != strings.TrimSpace(table.ID) ||
				len([]rune(table.ID)) > maxIdentifierLength || seen[table.ID] ||
				table.Seats < 1 || table.Seats > maxSeats {
				return nil, appfault.ErrInvalidInput
			}
			seen[table.ID] = true
		}
	}
	return floors, nil
}

// Plan makes venue_tables and venue_layouts match the floor plan being saved, in the caller's
// transaction, so the editor, bookings, bills and reports share one set of tables and IDs:
//   - each floor is an active layout (matched by name); floors no longer in the plan are deactivated;
//   - each table is upserted by its number and keeps its row ID, also when it comes back after
//     being removed;
//   - tables missing from the plan are soft-deleted, unless guests still need them (ErrTableInUse).
func Plan(ctx context.Context, tx pgx.Tx, venueID string, floors []PlanFloor) error {
	var locked string
	err := tx.QueryRow(ctx, `SELECT id::text FROM venues WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`,
		venueID).Scan(&locked)
	if errors.Is(err, pgx.ErrNoRows) {
		return appfault.ErrNotFound
	}
	if err != nil {
		return err
	}

	layoutIDs := map[string]string{}
	activeLayouts := []string{}
	identifiers := []string{}
	for _, floor := range floors {
		if _, ok := layoutIDs[floor.Name]; !ok {
			id, err := ensureLayout(ctx, tx, venueID, floor.Name)
			if err != nil {
				return err
			}
			layoutIDs[floor.Name] = id
			activeLayouts = append(activeLayouts, id)
		}
		for _, table := range floor.Tables {
			identifiers = append(identifiers, table.ID)
		}
	}
	if _, err = tx.Exec(ctx, `
		UPDATE venue_layouts SET is_active = (id::text = ANY($2::text[]))
		WHERE venue_id = $1 AND is_active IS DISTINCT FROM (id::text = ANY($2::text[]))`,
		venueID, activeLayouts); err != nil {
		return err
	}

	var inUse bool
	err = tx.QueryRow(ctx, `
		WITH removed AS (
		  SELECT id FROM venue_tables
		  WHERE venue_id = $1 AND deleted_at IS NULL AND NOT (identifier = ANY($2::text[]))
		  FOR UPDATE
		)
		SELECT EXISTS (
		  SELECT 1 FROM removed t WHERE `+inUseCondition+`
		)`, venueID, identifiers).Scan(&inUse)
	if err != nil {
		return err
	}
	if inUse {
		return ErrTableInUse
	}
	if _, err = tx.Exec(ctx, `
		UPDATE venue_tables SET deleted_at = now()
		WHERE venue_id = $1 AND deleted_at IS NULL AND NOT (identifier = ANY($2::text[]))`,
		venueID, identifiers); err != nil {
		return err
	}

	for _, floor := range floors {
		for _, table := range floor.Tables {
			if _, err = tx.Exec(ctx, upsertTable, venueID, layoutIDs[floor.Name], table.ID,
				table.Seats, table.dbShape(), table.X, table.Y); err != nil {
				return appfault.MapWriteError(err)
			}
		}
	}
	return nil
}

// inUseCondition: table t still has guests depending on it.
const inUseCondition = `(
  EXISTS (SELECT 1 FROM reservations r WHERE r.venue_table_id = t.id AND r.ends_at > now() AND (
    r.status = 'confirmed' OR
    (r.status = 'pending' AND (r.hold_expires_at IS NULL OR r.hold_expires_at > now()))))
  OR EXISTS (SELECT 1 FROM reservations r WHERE r.venue_table_id = t.id
    AND r.status = 'seated' AND r.starts_at > now() - interval '24 hours')
  OR EXISTS (SELECT 1 FROM bills b WHERE b.venue_table_id = t.id
    AND b.status IN ('open', 'payment_pending'))
  OR EXISTS (SELECT 1 FROM orders o WHERE o.venue_table_id = t.id
    AND o.status IN ('new', 'accepted', 'preparing', 'ready'))
)`

const upsertTable = `
	INSERT INTO venue_tables (venue_id, layout_id, identifier, seats, shape, position_x, position_y, qr_token_hash)
	VALUES ($1, $2, $3, $4, $5, $6, $7, 'unissued:' || gen_random_uuid()::text)
	ON CONFLICT (venue_id, identifier) DO UPDATE SET
	  layout_id = EXCLUDED.layout_id, seats = EXCLUDED.seats, shape = EXCLUDED.shape,
	  position_x = EXCLUDED.position_x, position_y = EXCLUDED.position_y, deleted_at = NULL`

// ensureLayout returns the ID of the venue's layout with this name, creating or reactivating it.
func ensureLayout(ctx context.Context, tx pgx.Tx, venueID, name string) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `
		INSERT INTO venue_layouts (venue_id, name, is_active) VALUES ($1, $2, true)
		ON CONFLICT (venue_id, name) DO UPDATE SET is_active = true
		RETURNING id::text`, venueID, name).Scan(&id)
	return id, err
}
