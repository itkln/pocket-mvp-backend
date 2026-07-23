package ordering

import "context"

type Repository interface {
	List(context.Context, string) ([]Order, error)
	UpdateStatus(context.Context, string, string, string) (Order, error)
}
