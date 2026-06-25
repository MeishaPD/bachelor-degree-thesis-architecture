package outbound

import (
	"context"
	"thesis/hexagonal-backend/internal/core/domain"
)

type ProductRepository interface {
	FindByID(ctx context.Context, id int64) (domain.ProductDetail, error)
}
