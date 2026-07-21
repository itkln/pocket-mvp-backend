package owner

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

type VenueInput struct {
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

type Dashboard struct {
	RevenueMinor      int64   `json:"revenue_minor"`
	OrdersToday       int     `json:"orders_today"`
	AverageOrderMinor int64   `json:"average_order_minor"`
	ActiveTables      int     `json:"active_tables"`
	TotalTables       int     `json:"total_tables"`
	NewOrders         int     `json:"new_orders"`
	AverageRating     float64 `json:"average_rating"`
}

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

type StaffMember struct {
	ID          string     `json:"id"`
	DisplayName string     `json:"display_name"`
	Email       string     `json:"email"`
	Role        string     `json:"role"`
	Status      string     `json:"status"`
	InvitedAt   time.Time  `json:"invited_at"`
	AcceptedAt  *time.Time `json:"accepted_at,omitempty"`
}

type StaffInput struct {
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	Status      string `json:"status"`
}

type Order struct {
	ID         string    `json:"id"`
	Number     int64     `json:"number"`
	Channel    string    `json:"channel"`
	Source     string    `json:"source"`
	GuestName  string    `json:"guest_name,omitempty"`
	TotalMinor int       `json:"total_minor"`
	Currency   string    `json:"currency"`
	Status     string    `json:"status"`
	Notes      string    `json:"notes,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type Review struct {
	ID         string     `json:"id"`
	GuestName  string     `json:"guest_name"`
	Rating     int        `json:"rating"`
	Table      string     `json:"table,omitempty"`
	Order      string     `json:"order,omitempty"`
	Body       string     `json:"body,omitempty"`
	OwnerReply string     `json:"owner_reply,omitempty"`
	RepliedAt  *time.Time `json:"replied_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type Payment struct {
	ID            string     `json:"id"`
	Provider      string     `json:"provider"`
	Status        string     `json:"status"`
	AmountMinor   int        `json:"amount_minor"`
	RefundedMinor int        `json:"refunded_minor"`
	Currency      string     `json:"currency"`
	PaidAt        *time.Time `json:"paid_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type Subscription struct {
	ID                string     `json:"id"`
	Plan              string     `json:"plan"`
	BillingCycle      string     `json:"billing_cycle"`
	Status            string     `json:"status"`
	VenueLimit        *int       `json:"venue_limit,omitempty"`
	TrialEndsAt       *time.Time `json:"trial_ends_at,omitempty"`
	CurrentPeriodEnds *time.Time `json:"current_period_ends_at,omitempty"`
}

type SubscriptionInput struct {
	Plan         string `json:"plan"`
	BillingCycle string `json:"billing_cycle"`
}
