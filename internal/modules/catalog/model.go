package catalog

type Category struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
	IsActive  bool   `json:"is_active"`
	ItemCount int    `json:"item_count"`
}

type CategoryInput struct {
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
	IsActive  *bool  `json:"is_active,omitempty"`
}

type MenuItem struct {
	ID          string `json:"id"`
	CategoryID  string `json:"category_id"`
	Category    string `json:"category"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	PriceMinor  int    `json:"price_minor"`
	Currency    string `json:"currency"`
	IsAvailable bool   `json:"is_available"`
	IsPopular   bool   `json:"is_popular"`
	SortOrder   int    `json:"sort_order"`
	ImageURL    string `json:"image_url,omitempty"`
}

type MenuItemInput struct {
	CategoryID  string `json:"category_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	PriceMinor  int    `json:"price_minor"`
	Currency    string `json:"currency"`
	IsAvailable *bool  `json:"is_available,omitempty"`
	IsPopular   bool   `json:"is_popular"`
	SortOrder   int    `json:"sort_order"`
	ImageURL    string `json:"image_url"`
}
