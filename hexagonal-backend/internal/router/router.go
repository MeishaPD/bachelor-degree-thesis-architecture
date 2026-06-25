package router

import (
	"net/http"

	httpadapter "thesis/hexagonal-backend/internal/adapter/inbound/http"
)

func NewRouter(
	ping *httpadapter.PingHandler,
	order *httpadapter.OrderHandler,
	pricing *httpadapter.PricingHandler,
	product *httpadapter.ProductHandler,
	customer *httpadapter.CustomerHandler,
) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", ping.Handle)
	mux.HandleFunc("POST /orders/preview", order.Preview)
	mux.HandleFunc("POST /pricing/calculate", pricing.Calculate)
	mux.HandleFunc("GET /products/{id}", product.GetByID)
	mux.HandleFunc("PUT /orders/{id}/status", order.UpdateStatus)
	mux.HandleFunc("GET /customers/{id}/order-summary", customer.GetOrderSummary)
	return mux
}
