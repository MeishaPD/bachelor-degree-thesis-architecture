package dto

type PricingCalculateRequest struct {
	CustomerTier string             `json:"customer_tier"`
	ShippingZone string             `json:"shipping_zone"`
	VoucherCode  string             `json:"voucher_code"`
	Items        []OrderItemRequest `json:"items"`
}

type PricingCalculateResponse struct {
	Subtotal         int64 `json:"subtotal"`
	TierDiscount     int64 `json:"tier_discount"`
	CategoryDiscount int64 `json:"category_discount"`
	VoucherDiscount  int64 `json:"voucher_discount"`
	TotalDiscount    int64 `json:"total_discount"`
	ShippingFee      int64 `json:"shipping_fee"`
	Tax              int64 `json:"tax"`
	GrandTotal       int64 `json:"grand_total"`
}
