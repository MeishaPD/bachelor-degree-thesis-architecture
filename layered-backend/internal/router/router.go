package router

import (
	"net/http"

	"thesis/layered-backend/internal/handler"
)

func NewRouter(
	ping *handler.PingHandler,
	order *handler.OrderHandler,
	pricing *handler.PricingHandler,
	product *handler.ProductHandler,
	customer *handler.CustomerHandler,
	salesAnalytics *handler.SalesAnalyticsHandler,
) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", ping.Handle)
	mux.HandleFunc("POST /orders/preview", order.Preview)
	mux.HandleFunc("POST /pricing/calculate", pricing.Calculate)
	mux.HandleFunc("GET /products/{id}", product.GetByID)
	mux.HandleFunc("PUT /orders/{id}/status", order.UpdateStatus)
	mux.HandleFunc("GET /customers/{id}/order-summary", customer.GetOrderSummary)
	mux.HandleFunc("GET /admin/analytics/sales-summary", salesAnalytics.GetSalesSummary)
	return mux
}
