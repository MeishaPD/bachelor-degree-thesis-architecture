package httpadapter

import (
	"errors"
	"net/http"
	"strconv"

	"thesis/hexagonal-backend/internal/core/domain"
	"thesis/hexagonal-backend/internal/core/port/inbound"
)

type ProductHandler struct {
	usecase inbound.ProductUseCase
}

func NewProductHandler(usecase inbound.ProductUseCase) *ProductHandler {
	return &ProductHandler{usecase: usecase}
}

func (h *ProductHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method tidak diizinkan")
		return
	}

	idParam := r.PathValue("id")

	productID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "product id tidak valid")
		return
	}

	output, err := h.usecase.GetByID(r.Context(), productID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "product tidak ditemukan")
			return
		}

		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, mapProductResponse(output))
}
