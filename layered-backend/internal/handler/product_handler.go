package handler

import (
	"errors"
	"net/http"
	"strconv"

	"thesis/layered-backend/internal/handler/dto"
	"thesis/layered-backend/internal/model"
	"thesis/layered-backend/internal/service"
)

type ProductHandler struct {
	service *service.ProductService
}

func NewProductHandler(service *service.ProductService) *ProductHandler {
	return &ProductHandler{service: service}
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

	output, err := h.service.GetByID(r.Context(), productID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			writeError(w, http.StatusNotFound, "product tidak ditemukan")
			return
		}

		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, dto.ProductResponse{
		ProductID: output.ID,
		SKU:       output.SKU,
		Name:      output.Name,
		Price:     output.Price,
		StockQty:  output.StockQty,
	})
}
