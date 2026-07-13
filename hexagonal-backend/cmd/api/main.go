package main

import (
	"log"
	"net/http"
	"os"
	"time"

	httpadapter "thesis/hexagonal-backend/internal/adapter/inbound/http"
	"thesis/hexagonal-backend/internal/adapter/outbound/memory"
	restadapter "thesis/hexagonal-backend/internal/adapter/outbound/rest"
	"thesis/hexagonal-backend/internal/core/domain"
	"thesis/hexagonal-backend/internal/core/usecase"
	"thesis/hexagonal-backend/internal/observability"
	"thesis/hexagonal-backend/internal/router"
	"thesis/hexagonal-backend/internal/seed"
)

func main() {
	seedPath := getenv("PRODUCT_SEED_PATH", "data/products_seed.json")
	port := getenv("PORT", "8082")
	pprofAddr := getenv("PPROF_ADDR", "")
	supabaseURL := getenv("SUPABASE_URL", "")
	supabaseAPIKey := getenv("SUPABASE_API_KEY", "")

	if pprofAddr != "" {
		observability.StartPprofServer(pprofAddr)
	}

	products, err := seed.LoadProducts(seedPath)
	if err != nil {
		log.Fatalf("load seed products: %v", err)
	}

	restClient, err := restadapter.NewClient(
		&http.Client{
			Timeout: 5 * time.Second,
		},
		supabaseURL,
		supabaseAPIKey,
	)
	if err != nil {
		log.Fatalf("initialize Supabase client: %v", err)
	}

	catalogAdapter := memory.NewCatalogAdapter(products)
	pricingPolicy := domain.NewDefaultPricingPolicy()

	productRepository := restadapter.NewProductRepository(restClient)
	orderRepository := restadapter.NewOrderRepository(restClient)
	customerRepository := restadapter.NewCustomerRepository(restClient)

	orderPreviewUseCase := usecase.NewOrderPreviewUseCase(catalogAdapter, pricingPolicy)
	orderStatusUseCase := usecase.NewOrderStatusUseCase(orderRepository)
	pricingUseCase := usecase.NewPricingCalculateUseCase(catalogAdapter, pricingPolicy)
	productUseCase := usecase.NewProductUseCase(productRepository)
	customerUseCase := usecase.NewCustomerOrderSummaryUseCase(customerRepository)

	pingHandler := httpadapter.NewPingHandler()
	orderHandler := httpadapter.NewOrderHandler(orderPreviewUseCase, orderStatusUseCase)
	pricingHandler := httpadapter.NewPricingHandler(pricingUseCase)
	productHandler := httpadapter.NewProductHandler(productUseCase)
	customerHandler := httpadapter.NewCustomerHandler(customerUseCase)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: router.NewRouter(pingHandler, orderHandler, pricingHandler, productHandler, customerHandler),
	}

	log.Printf("hexagonal backend running on :%s", port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
