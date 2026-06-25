package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"thesis/layered-backend/internal/handler/dto"
	"thesis/layered-backend/internal/model"
	"thesis/layered-backend/internal/service"
)

type OrderHandler struct {
	previewService *service.OrderPreviewService
	statusService  *service.OrderStatusService
}

func NewOrderHandler(
	previewService *service.OrderPreviewService,
	statusService *service.OrderStatusService,
) *OrderHandler {
	return &OrderHandler{
		previewService: previewService,
		statusService:  statusService,
	}
}

func (h *OrderHandler) Preview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method tidak diizinkan")
		return
	}

	var req dto.OrderPreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "payload tidak valid")
		return
	}

	resp, err := h.previewService.Preview(mapOrderPreviewRequest(req))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, mapOrderPreviewResult(resp))
}

func (h *OrderHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method tidak diizinkan")
		return
	}

	idParam := r.PathValue("id")

	orderID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "order id tidak valid")
		return
	}

	var req dto.UpdateOrderStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "payload tidak valid")
		return
	}

	output, err := h.statusService.UpdateStatus(r.Context(), orderID, req.Status)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			writeError(w, http.StatusNotFound, "order tidak ditemukan")
			return
		}

		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, mapUpdateOrderStatusResult(output))
}

func mapOrderPreviewRequest(output dto.OrderPreviewRequest) service.OrderPreviewRequest {
	items := make([]service.OrderItemRequest, 0, len(output.Items))

	for _, item := range output.Items {
		items = append(items, service.OrderItemRequest{
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
		})
	}

	return service.OrderPreviewRequest{
		CustomerTier: output.CustomerTier,
		Items:        items,
	}
}

func mapOrderPreviewResult(result model.OrderPreviewResult) dto.OrderPreviewResponse {
	items := make([]dto.OrderPreviewItemResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, dto.OrderPreviewItemResponse{
			ProductID: item.ProductID,
			Name:      item.Name,
			Category:  item.Category,
			Quantity:  item.Quantity,
			UnitPrice: item.UnitPrice,
			Subtotal:  item.Subtotal,
		})
	}
	return dto.OrderPreviewResponse{
		Items:      items,
		Subtotal:   result.Subtotal,
		Discount:   result.Discount,
		FinalTotal: result.FinalTotal,
	}
}

func mapUpdateOrderStatusResult(result model.OrderStatusResult) dto.UpdateOrderStatusResponse {
	return dto.UpdateOrderStatusResponse{
		OrderID: result.OrderID,
		Status:  result.Status,
	}
}
