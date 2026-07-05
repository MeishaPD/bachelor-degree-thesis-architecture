package csv

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"thesis/hexagonal-backend/internal/core/domain"
)

type CatalogAdapter struct {
	products map[int64]domain.Product
}

func NewCatalogAdapter(path string) (*CatalogAdapter, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open csv catalog: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read csv catalog: %w", err)
	}

	products := make(map[int64]domain.Product)

	for i, row := range records {
		if i == 0 {
			continue
		}
		if len(row) < 5 {
			return nil, fmt.Errorf("invalid csv row %d", i+1)
		}

		id, err := strconv.ParseInt(row[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid product_id at row %d: %w", i+1, err)
		}

		price, err := strconv.ParseInt(row[3], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid price at row %d: %w", i+1, err)
		}

		active, err := strconv.ParseBool(row[4])
		if err != nil {
			return nil, fmt.Errorf("invalid active at row %d: %w", i+1, err)
		}

		products[id] = domain.Product{
			ID:       id,
			Name:     row[1],
			Category: row[2],
			Price:    price,
			Active:   active,
		}
	}

	return &CatalogAdapter{products: products}, nil
}

func (a *CatalogAdapter) GetProductByID(id int64) (domain.Product, bool) {
	product, ok := a.products[id]
	return product, ok
}
