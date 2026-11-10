package reporting

import "pocket-mvp-backend/internal/money"

type Dashboard struct {
	RevenueMinor      money.Minor `json:"revenue_minor"`
	OrdersToday       int         `json:"orders_today"`
	AverageOrderMinor money.Minor `json:"average_order_minor"`
	ActiveTables      int         `json:"active_tables"`
	TotalTables       int         `json:"total_tables"`
	NewOrders         int         `json:"new_orders"`
	AverageRating     float64     `json:"average_rating"`
}
