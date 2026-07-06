package inbound

import (
	"context"
	"time"
)

type SalesSummaryInput struct {
	StartDate time.Time
	EndDate   time.Time
}

type SalesSummaryOutput struct {
	StartDate         string
	EndDate           string
	TotalOrders       int64
	GrossRevenue      int64
	TotalItems        int64
	AverageOrderValue int64
}

type SalesAnalyticsUseCase interface {
	GetSalesSummary(ctx context.Context, input SalesSummaryInput) (SalesSummaryOutput, error)
}
