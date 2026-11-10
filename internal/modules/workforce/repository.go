package workforce

import "context"

type Repository interface {
	List(context.Context, string) ([]StaffMember, error)
	Create(context.Context, string, string, StaffInput) (StaffMember, error)
	Update(context.Context, string, string, StaffInput) (StaffMember, error)
	Delete(context.Context, string, string) error
}
