package reporting

import "context"

type Repository interface {
	Dashboard(context.Context, string) (Dashboard, error)
}
