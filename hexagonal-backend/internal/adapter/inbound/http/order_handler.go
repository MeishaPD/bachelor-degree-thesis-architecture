package httpadapter

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	httpdto "thesis/hexagonal-backend/internal/adapter/inbound/http/dto"
	"thesis/hexagonal-backend/internal/core/domain"
	"thesis/hexagonal-backend/internal/core/port/inbound"
)

type OrderHandler struct {
	previewUseCase inbound.OrderPreviewUseCase
	statusUseCase  inbound.OrderStatusUseCase
}

func NewOrderHandler(
	previewUseCase inbound.OrderPreviewUseCase,
	statusUseCase inbound.OrderStatusUseCase,
) *OrderHandler {
	return &OrderHandler{
		previewUseCase: previewUseCase,
		statusUseCase:  statusUseCase,
	}
}

func (h *OrderHandler) Preview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method tidak diizinkan")
		return
	}

	var req httpdto.OrderPreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "payload tidak valid")
		return
	}

	output, err := h.previewUseCase.Execute(mapOrderPreviewRequest(req))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, mapOrderPreviewResponse(output))
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

	var req httpdto.UpdateOrderStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "payload tidak valid")
		return
	}

	output, err := h.statusUseCase.UpdateStatus(r.Context(), inbound.UpdateOrderStatusInput{
		OrderID: orderID,
		Status:  req.Status,
	})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "order tidak ditemukan")
			return
		}

		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, mapUpdateOrderStatusResponse(output))
}
