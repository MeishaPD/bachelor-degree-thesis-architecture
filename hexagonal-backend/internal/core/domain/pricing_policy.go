package domain

import "math"

type PricingPolicy struct {
	TaxRatePercent     int64
	MaxDiscountPercent int64
	FreeShippingMin    int64
}

func NewDefaultPricingPolicy() PricingPolicy {
	return PricingPolicy{
		TaxRatePercent:     11,
		MaxDiscountPercent: 20,
		FreeShippingMin:    2000000,
	}
}

func (p PricingPolicy) TierDiscount(tier string, subtotal int64) int64 {
	switch tier {
	case "silver":
		return subtotal * 5 / 100
	case "gold":
		return subtotal * 10 / 100
	default:
		return 0
	}
}

func (p PricingPolicy) CategoryDiscount(category string, subtotal int64) int64 {
	switch category {
	case "peripheral":
		return subtotal * 3 / 100
	case "accessory":
		return subtotal * 2 / 100
	default:
		return 0
	}
}

func (p PricingPolicy) VoucherDiscount(voucher string, subtotal int64) int64 {
	if voucher == "HEMAT10" {
		return subtotal * 10 / 100
	}
	return 0
}

func (p PricingPolicy) CapDiscount(discount, subtotal int64) int64 {
	maxDiscount := subtotal * p.MaxDiscountPercent / 100
	if discount > maxDiscount {
		return maxDiscount
	}
	return discount
}

func (p PricingPolicy) BulkDiscount(totalQuantity int, subtotal int64) int64 {
	if totalQuantity >= 10 {
		return subtotal * 2 / 100
	}
	return 0
}

func (p PricingPolicy) ShippingFee(zone string, subtotal int64) int64 {
	if subtotal >= p.FreeShippingMin {
		return 0
	}
	switch zone {
	case "metro":
		return 15000
	case "domestic":
		return 30000
	default:
		return 50000
	}
}

func (p PricingPolicy) Tax(amount int64) int64 {
	if amount < 0 {
		return 0
	}
	return amount * p.TaxRatePercent / 100
}

func (p PricingPolicy) RoundToNearestHundred(value int64) int64 {
	return int64(math.Round(float64(value)/100.0) * 100)
}

type PricingResult struct {
	Subtotal         int64
	TierDiscount     int64
	CategoryDiscount int64
	VoucherDiscount  int64
	TotalDiscount    int64
	ShippingFee      int64
	Tax              int64
	GrandTotal       int64
}
