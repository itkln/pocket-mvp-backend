package venues

import (
	"time"

	"pocket-mvp-backend/internal/money"
)

type Venue struct {
	AccessRole  string         `json:"access_role,omitempty"`
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
	LogoURL         string    `json:"logo_url,omitempty"`
	Rating          float64   `json:"rating"`
	ReviewCount     int       `json:"review_count"`
	Tags            []string  `json:"tags"`
	Filters         []string  `json:"filters"`
	PreorderEnabled bool      `json:"preorder_enabled"`
	CreatedAt       time.Time `json:"created_at"`
}

// PublicMenu exposes only visible, available items of a published venue.
type PublicMenuItem struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	Description    string      `json:"description,omitempty"`
	PriceMinor     money.Minor `json:"price_minor"`
	Currency       string      `json:"currency"`
	ImageURL       string      `json:"image_url,omitempty"`
	PortionWeightG *int        `json:"portion_weight_g,omitempty"`
	EnergyKcal     *float64    `json:"energy_kcal,omitempty"`
	ProteinG       *float64    `json:"protein_g,omitempty"`
	FatG           *float64    `json:"fat_g,omitempty"`
	CarbsG         *float64    `json:"carbs_g,omitempty"`
}

type PublicMenuCategory struct {
	ID              string           `json:"id"`
	Name            string           `json:"name"`
	PreorderEnabled bool             `json:"preorder_enabled"`
	Items           []PublicMenuItem `json:"items"`
}

// PublicReview omits order, table and guest identity from the public feed.
type PublicReview struct {
	ID         string    `json:"id"`
	Rating     int       `json:"rating"`
	Body       string    `json:"body,omitempty"`
	OwnerReply string    `json:"owner_reply,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}
