package catalog

import "pocket-mvp-backend/internal/money"

type Category struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	SortOrder       int    `json:"sort_order"`
	IsActive        bool   `json:"is_active"`
	PreorderEnabled bool   `json:"preorder_enabled"`
	ItemCount       int    `json:"item_count"`
}

type CategoryInput struct {
	Name            string `json:"name"`
	SortOrder       int    `json:"sort_order"`
	IsActive        *bool  `json:"is_active,omitempty"`
	PreorderEnabled *bool  `json:"preorder_enabled,omitempty"`
}

type MenuItem struct {
	ID             string      `json:"id"`
	CategoryID     string      `json:"category_id"`
	Category       string      `json:"category"`
	Name           string      `json:"name"`
	Description    string      `json:"description,omitempty"`
	PriceMinor     money.Minor `json:"price_minor"`
	Currency       string      `json:"currency"`
	IsAvailable    bool        `json:"is_available"`
	IsPopular      bool        `json:"is_popular"`
	SortOrder      int         `json:"sort_order"`
	ImageURL       string      `json:"image_url,omitempty"`
	PortionWeightG *int        `json:"portion_weight_g,omitempty"`
	EnergyKcal     *float64    `json:"energy_kcal,omitempty"`
	ProteinG       *float64    `json:"protein_g,omitempty"`
	FatG           *float64    `json:"fat_g,omitempty"`
	CarbsG         *float64    `json:"carbs_g,omitempty"`
}

type MenuItemInput struct {
	CategoryID     string      `json:"category_id"`
	Name           string      `json:"name"`
	Description    string      `json:"description"`
	PriceMinor     money.Minor `json:"price_minor"`
	Currency       string      `json:"currency"`
	IsAvailable    *bool       `json:"is_available,omitempty"`
	IsPopular      bool        `json:"is_popular"`
	SortOrder      int         `json:"sort_order"`
	ImageURL       string      `json:"image_url"`
	PortionWeightG *int        `json:"portion_weight_g"`
	EnergyKcal     *float64    `json:"energy_kcal"`
	ProteinG       *float64    `json:"protein_g"`
	FatG           *float64    `json:"fat_g"`
	CarbsG         *float64    `json:"carbs_g"`
}

type ReorderInput struct {
	IDs []string `json:"ids"`
}

type ReorderMenuItemsInput struct {
	CategoryID string   `json:"category_id"`
	IDs        []string `json:"ids"`
}
