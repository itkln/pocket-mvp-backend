package billing

import "context"

type Repository interface {
	ListPayments(context.Context, string) ([]Payment, error)
	GetSubscription(context.Context, string) (*Subscription, error)
	CountVenues(context.Context, string) (int, error)
	ReplaceSubscription(context.Context, string, SubscriptionInput, *int) (Subscription, error)
}
