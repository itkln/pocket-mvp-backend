package reservations

import (
	"context"
	"time"
)

type Availability interface {
	FindSlot(context.Context, string, time.Time, int) (Slot, error)
}

type Repository interface {
	ListCustomer(context.Context, string) ([]CustomerReservation, error)
	CancelCustomer(context.Context, string, string) (CustomerReservation, error)
	FindByKey(context.Context, string, string) (Result, string, bool, error)
	Create(context.Context, string, CreateInput, string, Slot) (Result, error)
	Hold(context.Context, string, CreateInput, string, Slot) (HoldResult, error)
	Confirm(context.Context, string, string, CreateInput, string, string) (Result, error)
	GetConfirmed(context.Context, string, string, string) (Result, error)
}
