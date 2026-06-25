package dto

type OrderItemRequest struct {
	ProductID int64 `json:"product_id"`
	Quantity  int   `json:"quantity"`
}

type OrderPreviewRequest struct {
	CustomerTier string             `json:"customer_tier"`
	Items        []OrderItemRequest `json:"items"`
}

type OrderPreviewItemResponse struct {
	ProductID int64  `json:"product_id"`
	Name      string `json:"name"`
	Category  string `json:"category"`
	Quantity  int    `json:"quantity"`
	UnitPrice int64  `json:"unit_price"`
	Subtotal  int64  `json:"subtotal"`
}

type OrderPreviewResponse struct {
	Items      []OrderPreviewItemResponse `json:"items"`
	Subtotal   int64                      `json:"subtotal"`
	Discount   int64                      `json:"discount"`
	FinalTotal int64                      `json:"final_total"`
}
