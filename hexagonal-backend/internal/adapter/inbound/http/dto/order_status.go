package dto

type UpdateOrderStatusRequest struct {
	Status string `json:"order_status"`
}

type UpdateOrderStatusResponse struct {
	OrderID int64  `json:"order_id"`
	Status  string `json:"order_status"`
}
