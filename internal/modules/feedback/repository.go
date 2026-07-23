package feedback

import "context"

type Repository interface {
	List(context.Context, string) ([]Review, error)
	Reply(context.Context, string, string, string, string) (Review, error)
}
