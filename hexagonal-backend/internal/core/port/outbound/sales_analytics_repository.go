package outbound

import (
	"context"
	"thesis/hexagonal-backend/internal/core/domain"
	"time"
)

type SalesAnalyticsRepository interface {
	GetSalesSummary(ctx context.Context, startDate time.Time, endDate time.Time) (domain.SalesSummary, error)
}
