package usecase

import (
	"errors"
	"fmt"

	"thesis/hexagonal-backend/internal/core/domain"
	"thesis/hexagonal-backend/internal/core/port/inbound"
	"thesis/hexagonal-backend/internal/core/port/outbound"
)

type OrderPreviewUseCase struct {
	catalog outbound.ProductCatalogReader
	policy  domain.PricingPolicy
}

func NewOrderPreviewUseCase(catalog outbound.ProductCatalogReader, policy domain.PricingPolicy) *OrderPreviewUseCase {
	return &OrderPreviewUseCase{catalog: catalog, policy: policy}
}

func (u *OrderPreviewUseCase) Execute(req inbound.OrderPreviewInput) (inbound.OrderPreviewOutput, error) {
	if len(req.Items) == 0 {
		return inbound.OrderPreviewOutput{}, errors.New("items tidak boleh kosong")
	}

	merged := make(map[int64]int, len(req.Items))
	for _, item := range req.Items {
		if item.Quantity <= 0 {
			return inbound.OrderPreviewOutput{}, fmt.Errorf("quantity untuk product_id %d harus lebih dari 0", item.ProductID)
		}
		merged[item.ProductID] += item.Quantity
	}

	result := domain.OrderPreviewResult{
		Items: make([]domain.OrderPreviewItem, 0, len(merged)),
	}

	for productID, qty := range merged {
		product, ok := u.catalog.GetProductByID(productID)
		if !ok {
			return inbound.OrderPreviewOutput{}, fmt.Errorf("product_id %d tidak ditemukan", productID)
		}

		orderItem, err := domain.NewOrderItem(product, qty)
		if err != nil {
			return inbound.OrderPreviewOutput{}, fmt.Errorf("product_id %d: %w", productID, err)
		}

		subtotal := orderItem.Subtotal()
		result.Subtotal += subtotal
		result.Items = append(result.Items, domain.OrderPreviewItem{
			ProductID: product.ID,
			Name:      product.Name,
			Category:  product.Category,
			Quantity:  qty,
			UnitPrice: product.Price,
			Subtotal:  subtotal,
		})
	}

	result.Discount = u.policy.TierDiscount(req.CustomerTier, result.Subtotal)
	result.FinalTotal = result.Subtotal - result.Discount
	if result.FinalTotal < 0 {
		result.FinalTotal = 0
	}

	return mapOrderPreviewResult(result), nil
}

func mapOrderPreviewResult(result domain.OrderPreviewResult) inbound.OrderPreviewOutput {
	items := make([]inbound.OrderPreviewItemOutput, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, inbound.OrderPreviewItemOutput{
			ProductID: item.ProductID,
			Name:      item.Name,
			Category:  item.Category,
			Quantity:  item.Quantity,
			UnitPrice: item.UnitPrice,
			Subtotal:  item.Subtotal,
		})
	}
	return inbound.OrderPreviewOutput{
		Items:      items,
		Subtotal:   result.Subtotal,
		Discount:   result.Discount,
		FinalTotal: result.FinalTotal,
	}
}
