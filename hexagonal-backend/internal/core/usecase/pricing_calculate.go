package usecase

import (
	"errors"
	"fmt"

	"thesis/hexagonal-backend/internal/core/domain"
	"thesis/hexagonal-backend/internal/core/port/inbound"
	"thesis/hexagonal-backend/internal/core/port/outbound"
)

type PricingCalculateUseCase struct {
	catalog outbound.ProductCatalogReader
	policy  domain.PricingPolicy
}

func NewPricingCalculateUseCase(catalog outbound.ProductCatalogReader, policy domain.PricingPolicy) *PricingCalculateUseCase {
	return &PricingCalculateUseCase{catalog: catalog, policy: policy}
}

func (u *PricingCalculateUseCase) Execute(req inbound.PricingCalculateInput) (inbound.PricingCalculateOutput, error) {
	if len(req.Items) == 0 {
		return inbound.PricingCalculateOutput{}, errors.New("items tidak boleh kosong")
	}

	var subtotal int64
	var categoryDiscount int64

	for _, item := range req.Items {
		if item.Quantity <= 0 {
			return inbound.PricingCalculateOutput{}, fmt.Errorf("quantity untuk product_id %d harus lebih dari 0", item.ProductID)
		}

		product, ok := u.catalog.GetProductByID(item.ProductID)
		if !ok {
			return inbound.PricingCalculateOutput{}, fmt.Errorf("product_id %d tidak ditemukan", item.ProductID)
		}

		orderItem, err := domain.NewOrderItem(product, item.Quantity)
		if err != nil {
			return inbound.PricingCalculateOutput{}, fmt.Errorf("product_id %d: %w", item.ProductID, err)
		}

		itemSubtotal := orderItem.Subtotal()
		subtotal += itemSubtotal
		categoryDiscount += u.policy.CategoryDiscount(product.Category, itemSubtotal)
	}

	tierDiscount := u.policy.TierDiscount(req.CustomerTier, subtotal)
	voucherDiscount := u.policy.VoucherDiscount(req.VoucherCode, subtotal)
	totalDiscount := u.policy.CapDiscount(tierDiscount+categoryDiscount+voucherDiscount, subtotal)

	taxableBase := subtotal - totalDiscount
	if taxableBase < 0 {
		taxableBase = 0
	}

	shippingFee := u.policy.ShippingFee(req.ShippingZone, subtotal)
	tax := u.policy.Tax(taxableBase)
	grandTotal := u.policy.RoundToNearestHundred(taxableBase + shippingFee + tax)

	result := domain.PricingResult{
		Subtotal:         subtotal,
		TierDiscount:     tierDiscount,
		CategoryDiscount: categoryDiscount,
		VoucherDiscount:  voucherDiscount,
		TotalDiscount:    totalDiscount,
		ShippingFee:      shippingFee,
		Tax:              tax,
		GrandTotal:       grandTotal,
	}
	return mapPricingResult(result), nil
}

func mapPricingResult(result domain.PricingResult) inbound.PricingCalculateOutput {
	return inbound.PricingCalculateOutput{
		Subtotal:         result.Subtotal,
		TierDiscount:     result.TierDiscount,
		CategoryDiscount: result.CategoryDiscount,
		VoucherDiscount:  result.VoucherDiscount,
		TotalDiscount:    result.TotalDiscount,
		ShippingFee:      result.ShippingFee,
		Tax:              result.Tax,
		GrandTotal:       result.GrandTotal,
	}
}
