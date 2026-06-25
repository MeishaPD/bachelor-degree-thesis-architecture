package dto

type CustomerOrderSummaryResponse struct {
	CustomerID  int64  `json:"customer_id"`
	FullName    string `json:"full_name"`
	TotalOrders int64  `json:"total_orders"`
	TotalSpent  int64  `json:"total_spent"`
	TotalItems  int64  `json:"total_items"`
}
