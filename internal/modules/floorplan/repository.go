package floorplan

import (
	"context"
	"encoding/json"
)

type Repository interface {
	Get(context.Context, string) (json.RawMessage, error)
	Save(context.Context, string, string, json.RawMessage) error
}
