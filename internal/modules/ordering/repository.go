package ordering

import "context"

type Repository interface {
	List(context.Context, string) ([]Order, error)
	UpdateStatus(context.Context, string, string, string) (Order, error)
	Quote(context.Context, string, CreateInput) (Quote, error)
	Submit(context.Context, string, CreateInput, string) (Order, error)
	ListCustomer(context.Context, string) ([]Order, error)
	GetPublic(context.Context, string, string, string) (Order, error)
}
