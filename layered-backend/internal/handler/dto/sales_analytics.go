package dto

type SalesSummaryResponse struct {
	StartDate         string `json:"start_date"`
	EndDate           string `json:"end_date"`
	TotalOrders       int64  `json:"total_orders"`
	GrossRevenue      int64  `json:"gross_revenue"`
	TotalItems        int64  `json:"total_items"`
	AverageOrderValue int64  `json:"average_order_value"`
}
