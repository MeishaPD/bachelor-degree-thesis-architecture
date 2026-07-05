package csv

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"thesis/layered-backend/internal/model"
)

type CatalogRepository struct {
	products map[int64]model.Product
}

func NewCatalogRepository(path string) (*CatalogRepository, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open csv catalog: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read csv records: %w", err)
	}

	products := make(map[int64]model.Product)

	for i, row := range records {
		if i == 0 {
			continue
		}
		if len(row) < 5 {
			return nil, fmt.Errorf("invalid csv row: %d", i+1)
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

		products[id] = model.Product{
			ID:       id,
			Name:     row[1],
			Category: row[2],
			Price:    price,
			Active:   active,
		}
	}

	return &CatalogRepository{products: products}, nil
}

func (r *CatalogRepository) GetProductByID(id int64) (model.Product, bool) {
	product, ok := r.products[id]
	return product, ok
}
