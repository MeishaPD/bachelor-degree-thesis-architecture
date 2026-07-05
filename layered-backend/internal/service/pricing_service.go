package service

import (
	"errors"
	"fmt"

	"thesis/layered-backend/internal/model"
	"thesis/layered-backend/internal/repository/memory"
)

type OrderItemRequest struct {
	ProductID int64
	Quantity  int
}

type PricingCalculateRequest struct {
	CustomerTier string
	ShippingZone string
	VoucherCode  string
	Items        []OrderItemRequest
}

type PricingService struct {
	catalog       *memory.CatalogRepository
	pricingPolicy model.PricingPolicy
}

func NewPricingService(
	catalog *memory.CatalogRepository,
	pricingPolicy model.PricingPolicy,
) *PricingService {
	return &PricingService{catalog: catalog, pricingPolicy: pricingPolicy}
}

func (s *PricingService) Calculate(req PricingCalculateRequest) (model.PricingResult, error) {
	if len(req.Items) == 0 {
		return model.PricingResult{}, errors.New("items tidak boleh kosong")
	}

	var subtotal int64
	var categoryDiscount int64
	var totalQuantity int

	for _, item := range req.Items {
		if item.Quantity <= 0 {
			return model.PricingResult{}, fmt.Errorf("quantity untuk product_id %d harus lebih dari 0", item.ProductID)
		}

		product, ok := s.catalog.GetProductByID(item.ProductID)
		if !ok {
			return model.PricingResult{}, fmt.Errorf("product_id %d tidak ditemukan", item.ProductID)
		}

		orderItem, err := model.NewOrderItem(product, item.Quantity)
		if err != nil {
			return model.PricingResult{}, fmt.Errorf("product_id %d: %w", item.ProductID, err)
		}

		totalQuantity += item.Quantity
		itemSubtotal := orderItem.Subtotal()
		subtotal += itemSubtotal
		categoryDiscount += s.pricingPolicy.CategoryDiscount(product.Category, itemSubtotal)
	}

	tierDiscount := s.pricingPolicy.TierDiscount(req.CustomerTier, subtotal)
	voucherDiscount := s.pricingPolicy.VoucherDiscount(req.VoucherCode, subtotal)
	bulkDiscount := s.pricingPolicy.BulkQuantityDiscount(totalQuantity, subtotal)
	totalDiscount := s.pricingPolicy.CapDiscount(tierDiscount+categoryDiscount+voucherDiscount+bulkDiscount, subtotal)

	taxableBase := subtotal - totalDiscount
	if taxableBase < 0 {
		taxableBase = 0
	}

	shippingFee := s.pricingPolicy.ShippingFee(req.ShippingZone, subtotal)
	tax := s.pricingPolicy.Tax(taxableBase)
	grandTotal := s.pricingPolicy.RoundToNearestHundred(taxableBase + shippingFee + tax)

	return model.PricingResult{
		Subtotal:         subtotal,
		TierDiscount:     tierDiscount,
		CategoryDiscount: categoryDiscount,
		VoucherDiscount:  voucherDiscount,
		TotalDiscount:    totalDiscount,
		ShippingFee:      shippingFee,
		Tax:              tax,
		GrandTotal:       grandTotal,
	}, nil
}
