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
