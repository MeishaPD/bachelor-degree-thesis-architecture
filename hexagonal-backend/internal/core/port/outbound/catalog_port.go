package outbound

import "thesis/hexagonal-backend/internal/core/domain"

type ProductCatalogReader interface {
	GetProductByID(id int64) (domain.Product, bool)
}
