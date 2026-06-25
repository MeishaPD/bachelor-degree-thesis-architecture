package outbound

import (
	"context"
	"thesis/hexagonal-backend/internal/core/domain"
)

type CustomerRepository interface {
	GetOrderSummary(ctx context.Context, customerID int64) (domain.CustomerOrderSummary, error)
}
