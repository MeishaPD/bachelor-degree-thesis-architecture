package handler

import (
	"encoding/json"
	"net/http"

	"thesis/layered-backend/internal/handler/dto"
	"thesis/layered-backend/internal/service"
)

type PricingHandler struct {
	service *service.PricingService
}

func NewPricingHandler(service *service.PricingService) *PricingHandler {
	return &PricingHandler{service: service}
}

func (h *PricingHandler) Calculate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method tidak diizinkan")
		return
	}

	var req dto.PricingCalculateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "payload tidak valid")
		return
	}

	resp, err := h.service.Calculate(mapPricingCalculateRequest(req))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, dto.PricingCalculateResponse{
		Subtotal:         resp.Subtotal,
		TierDiscount:     resp.TierDiscount,
		CategoryDiscount: resp.CategoryDiscount,
		VoucherDiscount:  resp.VoucherDiscount,
		TotalDiscount:    resp.TotalDiscount,
		ShippingFee:      resp.ShippingFee,
		Tax:              resp.Tax,
		GrandTotal:       resp.GrandTotal,
	})
}

func mapPricingCalculateRequest(req dto.PricingCalculateRequest) service.PricingCalculateRequest {
	items := make([]service.OrderItemRequest, 0, len(req.Items))

	for _, item := range req.Items {
		items = append(items, service.OrderItemRequest{
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
		})
	}

	return service.PricingCalculateRequest{
		CustomerTier: req.CustomerTier,
		ShippingZone: req.ShippingZone,
		VoucherCode:  req.VoucherCode,
		Items:        items,
	}
}
