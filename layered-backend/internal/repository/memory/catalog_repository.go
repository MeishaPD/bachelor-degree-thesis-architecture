package memory

import "thesis/layered-backend/internal/model"

type CatalogRepository struct {
	products map[int64]model.Product
}

func NewCatalogRepository(products map[int64]model.Product) *CatalogRepository {
	return &CatalogRepository{products: products}
}

func (r *CatalogRepository) GetProductByID(id int64) (model.Product, bool) {
	p, ok := r.products[id]
	return p, ok
}
