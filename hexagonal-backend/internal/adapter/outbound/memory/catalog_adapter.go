package memory

import "thesis/hexagonal-backend/internal/core/domain"

type CatalogAdapter struct {
	products map[int64]domain.Product
}

func NewCatalogAdapter(products map[int64]domain.Product) *CatalogAdapter {
	return &CatalogAdapter{products: products}
}

func (a *CatalogAdapter) GetProductByID(id int64) (domain.Product, bool) {
	p, ok := a.products[id]
	return p, ok
}
