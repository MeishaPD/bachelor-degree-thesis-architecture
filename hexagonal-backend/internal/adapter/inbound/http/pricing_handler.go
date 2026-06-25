package httpadapter

import (
	"encoding/json"
	"net/http"

	httpdto "thesis/hexagonal-backend/internal/adapter/inbound/http/dto"
	"thesis/hexagonal-backend/internal/core/port/inbound"
)

type PricingHandler struct {
	usecase inbound.PricingCalculateUseCase
}

func NewPricingHandler(usecase inbound.PricingCalculateUseCase) *PricingHandler {
	return &PricingHandler{usecase: usecase}
}

func (h *PricingHandler) Calculate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method tidak diizinkan")
		return
	}

	var req httpdto.PricingCalculateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "payload tidak valid")
		return
	}

	output, err := h.usecase.Execute(mapPricingCalculateRequest(req))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, mapPricingCalculateResponse(output))
}
