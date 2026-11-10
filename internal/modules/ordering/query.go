package ordering

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/validate"
	"sort"
	"strings"
	"time"
)

type Query struct {
	Search   string
	Table    string
	DateFrom string
	DateTo   string
	Scope    string
	Statuses []string
	Channel  string
	TableID  string
	Limit    int
	Cursor   string
	after    *orderCursor
}
type Page struct {
	Orders     []Order `json:"orders"`
	NextCursor string  `json:"next_cursor,omitempty"`
	HasMore    bool    `json:"has_more"`
}
type orderCursor struct {
	Version   int       `json:"v"`
	Key       string    `json:"key"`
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
}

// Queue scopes: orders still in service, or finished ones.
const (
	ScopeActive  = "active"
	ScopeHistory = "history"
)

func (q *Query) normalize(venueID string) error {
	if !validate.UUID(venueID) {
		return appfault.ErrInvalidInput
	}
	if q.Scope == "" {
		q.Scope = ScopeActive
	}
	if !validate.OneOf(q.Scope, ScopeActive, ScopeHistory) {
		return appfault.ErrInvalidInput
	}
	if q.Limit == 0 {
		q.Limit = 50
	}
	if q.Limit < 1 || q.Limit > 100 {
		return appfault.ErrInvalidInput
	}
	if q.Channel != "" && !isChannel(q.Channel) {
		return appfault.ErrInvalidInput
	}
	if q.TableID != "" && !validate.UUID(q.TableID) {
		return appfault.ErrInvalidInput
	}
	q.Search = strings.TrimSpace(q.Search)
	q.Table = strings.TrimSpace(q.Table)
	if len(q.Search) > 100 || len(q.Table) > 100 {
		return appfault.ErrInvalidInput
	}
	for _, date := range []string{q.DateFrom, q.DateTo} {
		if date != "" {
			if _, err := time.Parse("2006-01-02", date); err != nil {
				return appfault.ErrInvalidInput
			}
		}
	}
	if q.DateFrom != "" && q.DateTo != "" && q.DateFrom > q.DateTo {
		return appfault.ErrInvalidInput
	}
	q.TableID = strings.ToLower(q.TableID)
	unique := map[string]bool{}
	for _, status := range q.Statuses {
		if !isSettableStatus(status) || isFinalStatus(status) != (q.Scope == ScopeHistory) {
			return appfault.ErrInvalidInput
		}
		unique[status] = true
	}
	q.Statuses = nil
	for status := range unique {
		q.Statuses = append(q.Statuses, status)
	}
	sort.Strings(q.Statuses)
	q.after = nil
	if q.Cursor != "" {
		if len(q.Cursor) > 2048 {
			return appfault.ErrInvalidInput
		}
		raw, err := base64.RawURLEncoding.DecodeString(q.Cursor)
		if err != nil {
			return appfault.ErrInvalidInput
		}
		var cursor orderCursor
		if json.Unmarshal(raw, &cursor) != nil || cursor.Version != 1 || cursor.Key != q.key(venueID) || cursor.CreatedAt.IsZero() || !validate.UUID(cursor.ID) {
			return appfault.ErrInvalidInput
		}
		q.after = &cursor
	}
	return nil
}
func (q Query) key(venueID string) string {
	data, _ := json.Marshal([]any{strings.ToLower(venueID), q.Scope, q.Statuses, q.Channel, q.TableID, q.Search, q.Table, q.DateFrom, q.DateTo})
	return fmt.Sprintf("%x", sha256.Sum256(data))
}
func (q Query) cursorFor(venueID string, order Order) string {
	raw, _ := json.Marshal(orderCursor{1, q.key(venueID), order.CreatedAt, order.ID})
	return base64.RawURLEncoding.EncodeToString(raw)
}
