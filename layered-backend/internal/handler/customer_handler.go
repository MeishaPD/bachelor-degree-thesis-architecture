package handler

import (
	"errors"
	"net/http"
	"strconv"

	"thesis/layered-backend/internal/handler/dto"
	"thesis/layered-backend/internal/model"
	"thesis/layered-backend/internal/service"
)

type CustomerHandler struct {
	orderSummaryService *service.CustomerOrderSummaryService
}

func NewCustomerHandler(orderSummaryService *service.CustomerOrderSummaryService) *CustomerHandler {
	return &CustomerHandler{orderSummaryService: orderSummaryService}
}

func (h *CustomerHandler) GetOrderSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method tidak diizinkan")
		return
	}

	idParam := r.PathValue("id")
	customerID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "customer id tidak valid")
		return
	}

	output, err := h.orderSummaryService.GetOrderSummary(r.Context(), customerID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			writeError(w, http.StatusNotFound, "customer tidak ditemukan")
			return
		}

		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, dto.CustomerOrderSummaryResponse{
		CustomerID:  output.CustomerID,
		FullName:    output.FullName,
		TotalOrders: output.TotalOrders,
		TotalSpent:  output.TotalSpent,
		TotalItems:  output.TotalItems,
	})
}
