package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"thesis/layered-backend/internal/handler"
	"thesis/layered-backend/internal/model"
	"thesis/layered-backend/internal/observability"
	"thesis/layered-backend/internal/repository/memory"
	restrepository "thesis/layered-backend/internal/repository/rest"
	"thesis/layered-backend/internal/router"
	"thesis/layered-backend/internal/seed"
	"thesis/layered-backend/internal/service"
)

func main() {
	seedPath := getenv("PRODUCT_SEED_PATH", "data/products_seed.json")
	port := getenv("PORT", "8081")
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

	restClient, err := restrepository.NewClient(
		&http.Client{
			Timeout: 5 * time.Second,
		},
		supabaseURL,
		supabaseAPIKey,
	)
	if err != nil {
		log.Fatalf("initialize Supabase client: %v", err)
	}

	catalogRepository := memory.NewCatalogRepository(products)
	pricingPolicy := model.NewDefaultPricingPolicy()

	productRepository := restrepository.NewProductRepository(restClient)
	orderRepository := restrepository.NewOrderRepository(restClient)
	customerRepository := restrepository.NewCustomerRepository(restClient)

	orderPreviewService := service.NewOrderPreviewService(catalogRepository, pricingPolicy)
	orderStatusService := service.NewOrderStatusService(orderRepository)
	pricingService := service.NewPricingService(catalogRepository, pricingPolicy)
	productService := service.NewProductService(productRepository)
	customerService := service.NewCustomerOrderSummaryService(customerRepository)

	pingHandler := handler.NewPingHandler()
	orderHandler := handler.NewOrderHandler(orderPreviewService, orderStatusService)
	pricingHandler := handler.NewPricingHandler(pricingService)
	productHandler := handler.NewProductHandler(productService)
	customerHandler := handler.NewCustomerHandler(customerService)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: router.NewRouter(pingHandler, orderHandler, pricingHandler, productHandler, customerHandler),
	}

	log.Printf("layered backend running on :%s", port)
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
