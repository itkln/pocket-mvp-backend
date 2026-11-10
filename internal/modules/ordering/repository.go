package ordering

import "context"

type Repository interface {
	alertRepository
	ListPage(context.Context, string, Query) (Page, error)
	Get(context.Context, string, string) (Order, error)
	List(context.Context, string) ([]Order, error)
	ListCustomerPreorders(context.Context, string) ([]Order, error)
	UpdateStatus(context.Context, string, string, string, ...StatusChange) (Order, error)
}
