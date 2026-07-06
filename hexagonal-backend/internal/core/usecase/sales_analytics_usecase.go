package usecase

import (
	"context"
	"errors"
	"thesis/hexagonal-backend/internal/core/domain"
	"thesis/hexagonal-backend/internal/core/port/inbound"
	"thesis/hexagonal-backend/internal/core/port/outbound"
)

type SalesAnalyticsUseCase struct {
	repository outbound.SalesAnalyticsRepository
}

func NewSalesAnalyticsUseCase(repository outbound.SalesAnalyticsRepository) *SalesAnalyticsUseCase {
	return &SalesAnalyticsUseCase{repository: repository}
}

func (u *SalesAnalyticsUseCase) GetSalesSummary(ctx context.Context, input inbound.SalesSummaryInput) (inbound.SalesSummaryOutput, error) {
	if input.EndDate.Before(input.StartDate) {
		return inbound.SalesSummaryOutput{}, errors.New("end_date tidak boleh sebelum start_date")
	}

	summary, err := u.repository.GetSalesSummary(ctx, input.StartDate, input.EndDate)
	if err != nil {
		return inbound.SalesSummaryOutput{}, err
	}

	summary.StartDate = input.StartDate.Format("2006-01-02")
	summary.EndDate = input.EndDate.Format("2006-01-02")

	return mapSalesSummary(summary), nil
}

func mapSalesSummary(summary domain.SalesSummary) inbound.SalesSummaryOutput {
	return inbound.SalesSummaryOutput{
		StartDate:         summary.StartDate,
		EndDate:           summary.EndDate,
		TotalOrders:       summary.TotalOrders,
		GrossRevenue:      summary.GrossRevenue,
		TotalItems:        summary.TotalItems,
		AverageOrderValue: summary.AverageOrderValue,
	}
}
