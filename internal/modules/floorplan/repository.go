package floorplan

import (
	"context"
	"encoding/json"

	"pocket-mvp-backend/internal/tablesync"
)

// ErrTableInUse: the plan removes a table with an upcoming booking, seated guests, an open bill
// or an order in progress; nothing is saved.
var ErrTableInUse = tablesync.ErrTableInUse

type Repository interface {
	// Get returns the plan; each table carries table_id, its venue_tables ID.
	Get(context.Context, string) (json.RawMessage, error)
	// Save stores the plan together with the venue's table rows and returns it as Get would.
	Save(context.Context, string, json.RawMessage) (json.RawMessage, error)
}
