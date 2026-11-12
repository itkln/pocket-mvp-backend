package floorplan

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/tablesync"
)

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

type queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func (r *PostgresRepository) Get(ctx context.Context, venueID string) (json.RawMessage, error) {
	return load(ctx, r.db, venueID)
}

// Save stores the plan and, in the same transaction, makes the venue's table rows match it, so a
// plan is never saved without its tables (the service has already checked the floor capability).
func (r *PostgresRepository) Save(ctx context.Context, venueID string, plan json.RawMessage) (json.RawMessage, error) {
	floors, err := tablesync.ParsePlan(plan)
	if err != nil {
		return nil, err
	}
	stored, err := withoutTableIDs(plan)
	if err != nil {
		return nil, err
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = tablesync.Plan(ctx, tx, venueID, floors); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `
		UPDATE venues
		SET settings = jsonb_set(settings, '{floor_plan}', $2::jsonb, true)
		WHERE id = $1 AND deleted_at IS NULL`, venueID, []byte(stored)); err != nil {
		return nil, err
	}
	saved, err := load(ctx, tx, venueID)
	if err != nil {
		return nil, err
	}
	return saved, tx.Commit(ctx)
}

// load returns the stored plan with each table's venue_tables ID added as table_id: the ID
// bookings, bills and the guest app use for that table.
func load(ctx context.Context, q queryer, venueID string) (json.RawMessage, error) {
	var plan []byte
	err := q.QueryRow(ctx, `
		SELECT COALESCE(settings -> 'floor_plan', '[]'::jsonb)
		FROM venues
		WHERE id = $1 AND deleted_at IS NULL`, venueID).Scan(&plan)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, appfault.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := q.Query(ctx, `
		SELECT identifier, id::text FROM venue_tables
		WHERE venue_id = $1 AND deleted_at IS NULL`, venueID)
	if err != nil {
		return nil, err
	}
	ids, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) ([2]string, error) {
		var pair [2]string
		return pair, row.Scan(&pair[0], &pair[1])
	})
	if err != nil {
		return nil, err
	}
	byIdentifier := make(map[string]string, len(ids))
	for _, pair := range ids {
		byIdentifier[pair[0]] = pair[1]
	}
	return rewriteTables(plan, func(table map[string]json.RawMessage) {
		delete(table, "table_id")
		var identifier string
		if json.Unmarshal(table["id"], &identifier) != nil {
			return
		}
		if id, ok := byIdentifier[identifier]; ok {
			table["table_id"], _ = json.Marshal(id)
		}
	})
}

// withoutTableIDs drops the table_id the editor got back on load: venue_tables owns those IDs.
func withoutTableIDs(plan json.RawMessage) (json.RawMessage, error) {
	return rewriteTables(plan, func(table map[string]json.RawMessage) { delete(table, "table_id") })
}

// rewriteTables applies edit to every table of the plan, keeping all other fields as they are.
func rewriteTables(plan json.RawMessage, edit func(map[string]json.RawMessage)) (json.RawMessage, error) {
	var floors []map[string]json.RawMessage
	if err := json.Unmarshal(plan, &floors); err != nil {
		// A plan stored before plans were validated is returned as it is.
		return plan, nil
	}
	for _, floor := range floors {
		var tables []map[string]json.RawMessage
		if raw, ok := floor["tables"]; !ok || json.Unmarshal(raw, &tables) != nil {
			continue
		}
		for _, table := range tables {
			if table != nil {
				edit(table)
			}
		}
		encoded, err := json.Marshal(tables)
		if err != nil {
			return nil, err
		}
		floor["tables"] = encoded
	}
	if floors == nil {
		return json.RawMessage(`[]`), nil
	}
	return json.Marshal(floors)
}
