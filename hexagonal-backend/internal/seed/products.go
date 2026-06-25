package seed

import (
	"encoding/json"
	"fmt"
	"os"

	"thesis/hexagonal-backend/internal/core/domain"
)

type productSeed struct {
	ProductID int64  `json:"product_id"`
	Name      string `json:"name"`
	Category  string `json:"category"`
	Price     int64  `json:"price"`
	Active    bool   `json:"active"`
}

func LoadProducts(path string) (map[int64]domain.Product, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read products seed: %w", err)
	}

	var seeds []productSeed
	if err := json.Unmarshal(b, &seeds); err != nil {
		return nil, fmt.Errorf("parse products seed: %w", err)
	}

	catalog := make(map[int64]domain.Product, len(seeds))
	for _, p := range seeds {
		catalog[p.ProductID] = domain.Product{
			ID:       p.ProductID,
			Name:     p.Name,
			Category: p.Category,
			Price:    p.Price,
			Active:   p.Active,
		}
	}
	return catalog, nil
}
