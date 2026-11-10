package demodata

import (
	"pocket-mvp-backend/internal/money"
	"time"
)

const (
	demoOwnerID      = "10000000-0000-4000-8000-000000000001"
	mainVenueID      = "20000000-0000-4000-8000-000000000001"
	secondaryVenueID = "20000000-0000-4000-8000-000000000002"
)

type venueSeed struct {
	ID          string
	Slug        string
	Name        string
	Description string
	Cuisine     string
	Phone       string
	Email       string
	Address     string
	City        string
	PostalCode  string
	CountryCode string
	Timezone    string
	Currency    string
	Settings    map[string]any
}

type categorySeed struct {
	ID        string
	Name      string
	SortOrder int
	Active    bool
}

type itemSeed struct {
	ID          string
	CategoryID  string
	Name        string
	Description string
	PriceMinor  money.Minor
	Available   bool
	Popular     bool
	SortOrder   int
	ImageURL    string
}

type staffSeed struct {
	ID     string
	Name   string
	Email  string
	Role   string
	Status string
}

type tableSeed struct {
	ID         string
	LayoutID   string
	Identifier string
	Seats      int
	X          float64
	Y          float64
	Width      float64
	Height     float64
	Status     string
}

type fixtureSeed struct {
	ID       string
	LayoutID string
	Key      string
	Type     string
	X        float64
	Y        float64
	Rotation int
	Size     float64
}

type orderSeed struct {
	ID         string
	TableID    string
	StaffID    string
	Channel    string
	Status     string
	GuestName  string
	TotalMinor money.Minor
	Notes      string
	CreatedAt  time.Time
}

func demoVenues() []venueSeed {
	return []venueSeed{
		{
			ID: mainVenueID, Slug: "pocket-demo-bistro", Name: "Pocket Demo Bistro",
			Description: "Городское бистро с сезонным меню и открытой кухней.", Cuisine: "Современная европейская",
			Phone: "+421 910 555 101", Email: "bistro@pocket.local", Address: "Dunajská 12",
			City: "Bratislava", PostalCode: "811 08", CountryCode: "SK", Timezone: "Europe/Bratislava", Currency: "EUR",
			Settings: mainVenueSettings(),
		},
		{
			ID: secondaryVenueID, Slug: "pocket-demo-roastery", Name: "Pocket Coffee Lab",
			Description: "Небольшая кофейня и обжарочная лаборатория.", Cuisine: "Кофейня",
			Phone: "+421 910 555 202", Email: "coffee@pocket.local", Address: "Obchodná 24",
			City: "Bratislava", PostalCode: "811 06", CountryCode: "SK", Timezone: "Europe/Bratislava", Currency: "EUR",
			Settings: secondaryVenueSettings(),
		},
	}
}

func mainVenueSettings() map[string]any {
	return map[string]any{
		"cover_image_url":     "https://images.unsplash.com/photo-1517248135467-4c7edcad34c4?auto=format&fit=crop&w=1600&q=85",
		"public_category":     "restaurants",
		"public_filters":      []string{"Европейская", "Лучшие", "Открыто", "С предзаказом", "Рядом"},
		"public_tags":         []string{"Стол через 20 мин", "Предзаказ доступен"},
		"public_search_terms": "ресторан бистро европейская кухня",
		"schedule_enabled":    true, "same_day_booking": true,
		"dine_in_enabled": true, "preorder_enabled": true,
		"floor_plan": []map[string]any{
			{"id": "floor-main-1", "name": "1 этаж", "tables": []map[string]any{
				{"id": "01", "seats": 2, "x": 8, "y": 13, "state": "free"},
				{"id": "02", "seats": 4, "x": 31, "y": 12, "state": "busy"},
				{"id": "03", "seats": 4, "x": 59, "y": 14, "state": "reserved"},
				{"id": "04", "seats": 6, "x": 16, "y": 55, "state": "busy"},
				{"id": "05", "seats": 2, "x": 63, "y": 57, "state": "free"},
			}, "fixtures": []map[string]any{
				{"id": "bar-main", "type": "bar", "x": 33, "y": 86, "rotation": 0, "size": 120},
				{"id": "window-main", "type": "window", "x": 96, "y": 18, "rotation": 0, "size": 100},
				{"id": "entrance-main", "type": "entrance", "x": 5, "y": 84, "rotation": 0, "size": 100},
			}},
			{"id": "floor-main-2", "name": "2 этаж", "tables": []map[string]any{
				{"id": "11", "seats": 2, "x": 12, "y": 16, "state": "free"},
				{"id": "12", "seats": 4, "x": 43, "y": 17, "state": "reserved"},
				{"id": "13", "seats": 6, "x": 68, "y": 52, "state": "free"},
			}, "fixtures": []map[string]any{
				{"id": "window-second", "type": "window", "x": 94, "y": 20, "rotation": 0, "size": 110},
			}},
			{"id": "floor-main-3", "name": "Терраса", "tables": []map[string]any{
				{"id": "21", "seats": 2, "x": 15, "y": 25, "state": "free"},
				{"id": "22", "seats": 2, "x": 45, "y": 25, "state": "free"},
				{"id": "23", "seats": 4, "x": 68, "y": 58, "state": "busy"},
			}, "fixtures": []map[string]any{}},
		},
	}
}

func secondaryVenueSettings() map[string]any {
	return map[string]any{
		"cover_image_url":     "https://images.unsplash.com/photo-1445116572660-236099ec97a0?auto=format&fit=crop&w=1600&q=85",
		"public_category":     "restaurants",
		"public_filters":      []string{"Кофе и десерты", "Лучшие", "Открыто", "Рядом"},
		"public_tags":         []string{"Открыто сейчас", "Столик у окна"},
		"public_search_terms": "кофейня кофе десерты обжарка",
		"schedule_enabled":    true, "same_day_booking": false,
		"dine_in_enabled": true, "preorder_enabled": false,
		"floor_plan": []map[string]any{{"id": "floor-coffee-1", "name": "Основной зал", "tables": []map[string]any{
			{"id": "C1", "seats": 2, "x": 15, "y": 20, "state": "free"},
			{"id": "C2", "seats": 2, "x": 48, "y": 20, "state": "busy"},
			{"id": "C3", "seats": 4, "x": 65, "y": 58, "state": "free"},
		}, "fixtures": []map[string]any{{"id": "coffee-bar", "type": "bar", "x": 28, "y": 82, "rotation": 0, "size": 130}}}},
	}
}
