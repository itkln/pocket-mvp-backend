package dinein

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"pocket-mvp-backend/internal/appfault"
)

type HistoryInput struct {
	VenueID        string   `json:"venue_id"`
	LegacyKeys     []string `json:"legacy_keys"`
	Cursor         string   `json:"cursor"`
	CustomerUserID string   `json:"-"`
	GuestToken     string   `json:"-"`
}

type HistoryQuery struct {
	VenueID, CustomerUserID, GuestHash string
	LegacyKeys                         []string
	Before                             *historyCursor
}

type historyCursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
}

type HistoryPage struct {
	Scope      string  `json:"scope"`
	ScopeKey   string  `json:"scope_key"`
	Orders     []Order `json:"orders"`
	NextCursor string  `json:"next_cursor,omitempty"`
}

func (s *Service) History(ctx context.Context, input HistoryInput) (HistoryPage, error) {
	if !uuidPattern.MatchString(input.VenueID) || len(input.LegacyKeys) > 100 || len(input.Cursor) > 512 {
		return HistoryPage{}, appfault.ErrInvalidInput
	}
	query := HistoryQuery{VenueID: input.VenueID, CustomerUserID: input.CustomerUserID}
	if input.CustomerUserID == "" {
		if !validToken(input.GuestToken) {
			return HistoryPage{}, appfault.ErrInvalidInput
		}
		query.GuestHash = tokenHash(input.GuestToken)
	}
	for _, key := range input.LegacyKeys {
		if len(key) < 16 || len(key) > 128 || strings.TrimSpace(key) != key {
			return HistoryPage{}, appfault.ErrInvalidInput
		}
		query.LegacyKeys = append(query.LegacyKeys, key)
	}
	if input.Cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(input.Cursor)
		var cursor historyCursor
		if err != nil || json.Unmarshal(raw, &cursor) != nil || cursor.CreatedAt.IsZero() || !uuidPattern.MatchString(cursor.ID) {
			return HistoryPage{}, appfault.ErrInvalidInput
		}
		query.Before = &cursor
	}
	page, err := s.repository.History(ctx, query)
	page.Scope = "device"
	page.ScopeKey = tokenHash("device:" + query.GuestHash)
	if input.CustomerUserID != "" {
		page.Scope = "account"
		page.ScopeKey = tokenHash("account:" + input.CustomerUserID)
	}
	return page, err
}
