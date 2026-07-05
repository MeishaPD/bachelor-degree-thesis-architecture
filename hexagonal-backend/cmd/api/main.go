package main

import (
	"log"
	"net/http"
	"os"

	httpadapter "thesis/hexagonal-backend/internal/adapter/inbound/http"
	csvadapter "thesis/hexagonal-backend/internal/adapter/outbound/csv"
	"thesis/hexagonal-backend/internal/adapter/outbound/memory"
	"thesis/hexagonal-backend/internal/adapter/outbound/postgres"
	"thesis/hexagonal-backend/internal/core/domain"
	"thesis/hexagonal-backend/internal/core/port/outbound"
	"thesis/hexagonal-backend/internal/core/usecase"
	dbconn "thesis/hexagonal-backend/internal/db"
	"thesis/hexagonal-backend/internal/observability"
	"thesis/hexagonal-backend/internal/router"
	"thesis/hexagonal-backend/internal/seed"
)

func main() {
	var catalogAdapter outbound.ProductCatalogReader
	var err error

	seedPath := getenv("PRODUCT_SEED_PATH", "data/products_seed.json")
	port := getenv("PORT", "8082")
	pprofAddr := getenv("PPROF_ADDR", "")

	catalogSource := getenv("PRODUCT_CATALOG_SOURCE", "json")
	csvPath := getenv("PRODUCT_CSV_PATH", "data/products_seed.csv")

	switch catalogSource {
	case "csv":
		catalogAdapter, err = csvadapter.NewCatalogAdapter(csvPath)
		if err != nil {
			log.Fatalf("load csv catalog: %v", err)
		}
	default:
		products, err := seed.LoadProducts(seedPath)
		if err != nil {
			log.Fatalf("load seed products: %v", err)
		}
		catalogAdapter = memory.NewCatalogAdapter(products)
	}

	databaseURL := getenv("DATABASE_URL", "postgres://postgres:postMeisha2025.@localhost:5432/hexagonal-skripshit?sslmode=disable")

	if pprofAddr != "" {
		observability.StartPprofServer(pprofAddr)
	}

	db, err := dbconn.OpenPostgresPool(databaseURL)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer db.Close()

	pricingPolicy := domain.NewDefaultPricingPolicy()

	productRepository := postgres.NewProductRepository(db)
	orderRepository := postgres.NewOrderRepository(db)
	customerRepository := postgres.NewCustomerRepository(db)

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
