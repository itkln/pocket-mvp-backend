package dinein

import (
	"context"

	"pocket-mvp-backend/internal/access"
)

type TokenFactory func() (encrypted, hash string, err error)
type Repository interface {
	CurrentBill(context.Context, string) (*Bill, error)
	IssueQR(context.Context, string, string, bool, TokenFactory) (QRCode, error)
	Resolve(context.Context, string) (Table, error)
	Create(context.Context, string, string, CreateInput) (Order, error)
	Lookup(context.Context, string, string, string, string) (Order, error)
	History(context.Context, HistoryQuery) (HistoryPage, error)
}
type Authorizer interface {
	Require(context.Context, string, string, access.Capability) error
}
type Protector interface {
	Encrypt(string, string) (string, error)
	Decrypt(string, string) (string, error)
}
