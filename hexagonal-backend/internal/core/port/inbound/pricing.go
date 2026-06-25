package inbound

type PricingCalculateInput struct {
	CustomerTier string
	ShippingZone string
	VoucherCode  string
	Items        []OrderItemInput
}

type PricingCalculateOutput struct {
	Subtotal         int64
	TierDiscount     int64
	CategoryDiscount int64
	VoucherDiscount  int64
	TotalDiscount    int64
	ShippingFee      int64
	Tax              int64
	GrandTotal       int64
}

type PricingCalculateUseCase interface {
	Execute(input PricingCalculateInput) (PricingCalculateOutput, error)
}
