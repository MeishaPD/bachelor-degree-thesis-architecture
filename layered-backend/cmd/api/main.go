package main

import (
	"log"
	"net/http"
	"os"

	dbconn "thesis/layered-backend/internal/db"
	"thesis/layered-backend/internal/handler"
	"thesis/layered-backend/internal/model"
	"thesis/layered-backend/internal/observability"
	csvrepo "thesis/layered-backend/internal/repository/csv"
	"thesis/layered-backend/internal/repository/memory"
	"thesis/layered-backend/internal/repository/postgres"
	"thesis/layered-backend/internal/router"
	"thesis/layered-backend/internal/seed"
	"thesis/layered-backend/internal/service"
)

func main() {
	var catalogRepository service.CatalogReader
	var err error

	seedPath := getenv("PRODUCT_SEED_PATH", "data/products_seed.json")
	port := getenv("PORT", "8081")
	pprofAddr := getenv("PPROF_ADDR", "")

	databaseURL := getenv("DATABASE_URL", "postgres://postgres:postMeisha2025.@localhost:5432/hexagonal-skripshit?sslmode=disable")

	catalogSource := getenv("PRODUCT_CATALOG_SOURCE", "json")
	csvPath := getenv("PRODUCT_CSV_PATH", "data/products_seed.csv")

	switch catalogSource {
	case "csv":
		catalogRepository, err = csvrepo.NewCatalogRepository(csvPath)
		if err != nil {
			log.Fatalf("load csv catalog: %v", err)
		}
	default:
		products, err := seed.LoadProducts(seedPath)
		if err != nil {
			log.Fatalf("load seed products: %v", err)
		}
		catalogRepository = memory.NewCatalogRepository(products)
	}

	if pprofAddr != "" {
		observability.StartPprofServer(pprofAddr)
	}

	db, err := dbconn.OpenPostgresPool(databaseURL)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer db.Close()

	pricingPolicy := model.NewDefaultPricingPolicy()

	productRepository := postgres.NewProductRepository(db)
	orderRepository := postgres.NewOrderRepository(db)
	customerRepository := postgres.NewCustomerRepository(db)

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
