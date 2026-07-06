package handler

import (
	"net/http"
	"thesis/layered-backend/internal/handler/dto"
	"thesis/layered-backend/internal/service"
	"time"
)

type SalesAnalyticsHandler struct {
	service *service.SalesAnalyticsService
}

func NewSalesAnalyticsHandler(service *service.SalesAnalyticsService) *SalesAnalyticsHandler {
	return &SalesAnalyticsHandler{service: service}
}

func (h *SalesAnalyticsHandler) GetSalesSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method tidak diizinkan")
		return
	}

	startDate, endDate, ok := parseDateRange(w, r)
	if !ok {
		return
	}

	output, err := h.service.GetSalesSummary(r.Context(), startDate, endDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, dto.SalesSummaryResponse{
		StartDate:         output.StartDate,
		EndDate:           output.EndDate,
		TotalOrders:       output.TotalOrders,
		GrossRevenue:      output.GrossRevenue,
		TotalItems:        output.TotalItems,
		AverageOrderValue: output.AverageOrderValue,
	})
}

func parseDateRange(w http.ResponseWriter, r *http.Request) (time.Time, time.Time, bool) {
	const layout = "2006-01-02"

	startParam := r.URL.Query().Get("start_date")
	endParam := r.URL.Query().Get("end_date")

	if startParam == "" || endParam == "" {
		writeError(w, http.StatusBadRequest, "start_date dan end_date wajib diisi")
		return time.Time{}, time.Time{}, false
	}

	startDate, err := time.Parse(layout, startParam)
	if err != nil {
		writeError(w, http.StatusBadRequest, "format start_date harus YYYY-MM-DD")
		return time.Time{}, time.Time{}, false
	}

	endDate, err := time.Parse(layout, endParam)
	if err != nil {
		writeError(w, http.StatusBadRequest, "format end_date harus YYYY-MM-DD")
		return time.Time{}, time.Time{}, false
	}
	return startDate, endDate, true
}
