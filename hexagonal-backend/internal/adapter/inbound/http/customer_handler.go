package httpadapter

import (
	"errors"
	"net/http"
	"strconv"

	"thesis/hexagonal-backend/internal/core/domain"
	"thesis/hexagonal-backend/internal/core/port/inbound"
)

type CustomerHandler struct {
	usecase inbound.CustomerOrderSummaryUseCase
}

func NewCustomerHandler(usecase inbound.CustomerOrderSummaryUseCase) *CustomerHandler {
	return &CustomerHandler{usecase: usecase}
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

	output, err := h.usecase.GetOrderSummary(r.Context(), customerID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "customer tidak ditemukan")
			return
		}

		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, mapCustomerOrderSummaryResponse(output))
}
