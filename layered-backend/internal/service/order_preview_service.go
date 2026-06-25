package service

import (
	"errors"
	"fmt"

	"thesis/layered-backend/internal/model"
	"thesis/layered-backend/internal/repository/memory"
)

type OrderPreviewRequest struct {
	CustomerTier string
	Items        []OrderItemRequest
}

type OrderPreviewService struct {
	catalog *memory.CatalogRepository
	policy  model.PricingPolicy
}

func NewOrderPreviewService(
	catalog *memory.CatalogRepository,
	policy model.PricingPolicy,
) *OrderPreviewService {
	return &OrderPreviewService{catalog: catalog, policy: policy}
}

func (s *OrderPreviewService) Preview(req OrderPreviewRequest) (model.OrderPreviewResult, error) {
	if len(req.Items) == 0 {
		return model.OrderPreviewResult{}, errors.New("items tidak boleh kosong")
	}

	merged := make(map[int64]int, len(req.Items))
	for _, item := range req.Items {
		if item.Quantity <= 0 {
			return model.OrderPreviewResult{}, fmt.Errorf("quantity untuk product_id %d harus lebih dari 0", item.ProductID)
		}
		merged[item.ProductID] += item.Quantity
	}

	result := model.OrderPreviewResult{
		Items: make([]model.OrderPreviewItem, 0, len(merged)),
	}

	for productID, qty := range merged {
		product, ok := s.catalog.GetProductByID(productID)
		if !ok {
			return model.OrderPreviewResult{}, fmt.Errorf("product_id %d tidak ditemukan", productID)
		}

		orderItem, err := model.NewOrderItem(product, qty)
		if err != nil {
			return model.OrderPreviewResult{}, fmt.Errorf("product_id %d: %w", productID, err)
		}

		subtotal := orderItem.Subtotal()
		result.Subtotal += subtotal
		result.Items = append(result.Items, model.OrderPreviewItem{
			ProductID: product.ID,
			Name:      product.Name,
			Category:  product.Category,
			Quantity:  qty,
			UnitPrice: product.Price,
			Subtotal:  subtotal,
		})
	}

	result.Discount = s.policy.TierDiscount(req.CustomerTier, result.Subtotal)
	result.FinalTotal = result.Subtotal - result.Discount
	if result.FinalTotal < 0 {
		result.FinalTotal = 0
	}

	return result, nil
}
