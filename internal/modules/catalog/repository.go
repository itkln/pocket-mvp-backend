package catalog

import "context"

type Repository interface {
	ListCategories(context.Context, string) ([]Category, error)
	CreateCategory(context.Context, string, CategoryInput) (Category, error)
	UpdateCategory(context.Context, string, string, CategoryInput) (Category, error)
	DeleteCategory(context.Context, string, string) error
	ReorderCategories(context.Context, string, []string) error
	ListMenuItems(context.Context, string) ([]MenuItem, error)
	CreateMenuItem(context.Context, string, MenuItemInput) (MenuItem, error)
	UpdateMenuItem(context.Context, string, string, MenuItemInput) (MenuItem, error)
	DeleteMenuItem(context.Context, string, string) error
	ReorderMenuItems(context.Context, string, string, []string) error
}
