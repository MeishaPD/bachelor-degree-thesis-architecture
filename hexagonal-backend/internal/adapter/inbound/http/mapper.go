package httpadapter

import (
	httpdto "thesis/hexagonal-backend/internal/adapter/inbound/http/dto"
	"thesis/hexagonal-backend/internal/core/port/inbound"
)

func mapOrderItems(items []httpdto.OrderItemRequest) []inbound.OrderItemInput {
	mapped := make([]inbound.OrderItemInput, 0, len(items))
	for _, item := range items {
		mapped = append(mapped, inbound.OrderItemInput{
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
		})
	}
	return mapped
}

func mapOrderPreviewRequest(req httpdto.OrderPreviewRequest) inbound.OrderPreviewInput {
	return inbound.OrderPreviewInput{
		CustomerTier: req.CustomerTier,
		Items:        mapOrderItems(req.Items),
	}
}

func mapOrderPreviewResponse(output inbound.OrderPreviewOutput) httpdto.OrderPreviewResponse {
	items := make([]httpdto.OrderPreviewItemResponse, 0, len(output.Items))
	for _, item := range output.Items {
		items = append(items, httpdto.OrderPreviewItemResponse{
			ProductID: item.ProductID,
			Name:      item.Name,
			Category:  item.Category,
			Quantity:  item.Quantity,
			UnitPrice: item.UnitPrice,
			Subtotal:  item.Subtotal,
		})
	}
	return httpdto.OrderPreviewResponse{
		Items:      items,
		Subtotal:   output.Subtotal,
		Discount:   output.Discount,
		FinalTotal: output.FinalTotal,
	}
}

func mapPricingCalculateRequest(req httpdto.PricingCalculateRequest) inbound.PricingCalculateInput {
	return inbound.PricingCalculateInput{
		CustomerTier: req.CustomerTier,
		ShippingZone: req.ShippingZone,
		VoucherCode:  req.VoucherCode,
		Items:        mapOrderItems(req.Items),
	}
}

func mapPricingCalculateResponse(output inbound.PricingCalculateOutput) httpdto.PricingCalculateResponse {
	return httpdto.PricingCalculateResponse{
		Subtotal:         output.Subtotal,
		TierDiscount:     output.TierDiscount,
		CategoryDiscount: output.CategoryDiscount,
		VoucherDiscount:  output.VoucherDiscount,
		TotalDiscount:    output.TotalDiscount,
		ShippingFee:      output.ShippingFee,
		Tax:              output.Tax,
		GrandTotal:       output.GrandTotal,
	}
}

func mapProductResponse(output inbound.ProductOutput) httpdto.ProductResponse {
	return httpdto.ProductResponse{
		ProductID: output.ProductID,
		SKU:       output.SKU,
		Name:      output.Name,
		Price:     output.Price,
		StockQty:  output.StockQty,
	}
}

func mapUpdateOrderStatusResponse(output inbound.UpdateOrderStatusOutput) httpdto.UpdateOrderStatusResponse {
	return httpdto.UpdateOrderStatusResponse{
		OrderID: output.OrderID,
		Status:  output.Status,
	}
}

func mapCustomerOrderSummaryResponse(output inbound.CustomerOrderSummaryOutput) httpdto.CustomerOrderSummaryResponse {
	return httpdto.CustomerOrderSummaryResponse{
		CustomerID:  output.CustomerID,
		FullName:    output.FullName,
		TotalOrders: output.TotalOrders,
		TotalSpent:  output.TotalSpent,
		TotalItems:  output.TotalItems,
	}
}
