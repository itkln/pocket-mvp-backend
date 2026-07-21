package reporting

type Dashboard struct {
	RevenueMinor      int64   `json:"revenue_minor"`
	OrdersToday       int     `json:"orders_today"`
	AverageOrderMinor int64   `json:"average_order_minor"`
	ActiveTables      int     `json:"active_tables"`
	TotalTables       int     `json:"total_tables"`
	NewOrders         int     `json:"new_orders"`
	AverageRating     float64 `json:"average_rating"`
}
