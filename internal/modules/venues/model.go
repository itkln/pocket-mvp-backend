package venues

import "time"

type Venue struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Slug        string         `json:"slug"`
	Description string         `json:"description,omitempty"`
	CuisineType string         `json:"cuisine_type,omitempty"`
	Phone       string         `json:"phone,omitempty"`
	Email       string         `json:"email,omitempty"`
	Address     string         `json:"address"`
	City        string         `json:"city"`
	PostalCode  string         `json:"postal_code,omitempty"`
	CountryCode string         `json:"country_code"`
	Timezone    string         `json:"timezone"`
	Currency    string         `json:"currency"`
	Status      string         `json:"status"`
	Settings    map[string]any `json:"settings"`
	CreatedAt   time.Time      `json:"created_at"`
}

type Input struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	CuisineType string         `json:"cuisine_type"`
	Phone       string         `json:"phone"`
	Email       string         `json:"email"`
	Address     string         `json:"address"`
	City        string         `json:"city"`
	PostalCode  string         `json:"postal_code"`
	CountryCode string         `json:"country_code"`
	Timezone    string         `json:"timezone"`
	Currency    string         `json:"currency"`
	Status      string         `json:"status"`
	Settings    map[string]any `json:"settings"`
}

// PublicVenue is the read model exposed to Pocket Spots. It deliberately
// excludes owner contact details and private venue settings.
type PublicVenue struct {
	ID              string    `json:"id"`
	Slug            string    `json:"slug"`
	Name            string    `json:"name"`
	Description     string    `json:"description,omitempty"`
	CategorySlug    string    `json:"category_slug"`
	CuisineType     string    `json:"cuisine_type,omitempty"`
	Address         string    `json:"address"`
	City            string    `json:"city"`
	CountryCode     string    `json:"country_code"`
	ImageURL        string    `json:"image_url,omitempty"`
	Rating          float64   `json:"rating"`
	ReviewCount     int       `json:"review_count"`
	Tags            []string  `json:"tags"`
	Filters         []string  `json:"filters"`
	PreorderEnabled bool      `json:"preorder_enabled"`
	CreatedAt       time.Time `json:"created_at"`
}

type PublicVenueQuery struct {
	City     string
	Search   string
	Category string
	Filter   string
	IDs      []string
	Page     int
	PageSize int
}

type PublicVenuePage struct {
	Venues     []PublicVenue `json:"venues"`
	Page       int           `json:"page"`
	PageSize   int           `json:"page_size"`
	Total      int           `json:"total"`
	TotalPages int           `json:"total_pages"`
	HasNext    bool          `json:"has_next"`
}

// PublicMenu exposes only visible, available items of a published venue.
type PublicMenuItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	PriceMinor  int    `json:"price_minor"`
	Currency    string `json:"currency"`
	ImageURL    string `json:"image_url,omitempty"`
}

type PublicMenuCategory struct {
	ID    string           `json:"id"`
	Name  string           `json:"name"`
	Items []PublicMenuItem `json:"items"`
}
