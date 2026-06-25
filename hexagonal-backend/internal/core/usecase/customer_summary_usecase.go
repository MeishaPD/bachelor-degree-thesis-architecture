package usecase

import (
	"context"

	"thesis/hexagonal-backend/internal/core/domain"
	"thesis/hexagonal-backend/internal/core/port/inbound"
	"thesis/hexagonal-backend/internal/core/port/outbound"
)

type CustomerOrderSummaryUseCase struct {
	repository outbound.CustomerRepository
}

func NewCustomerOrderSummaryUseCase(repository outbound.CustomerRepository) *CustomerOrderSummaryUseCase {
	return &CustomerOrderSummaryUseCase{repository: repository}
}

func (u *CustomerOrderSummaryUseCase) GetOrderSummary(ctx context.Context, customerID int64) (inbound.CustomerOrderSummaryOutput, error) {
	summary, err := u.repository.GetOrderSummary(ctx, customerID)
	if err != nil {
		return inbound.CustomerOrderSummaryOutput{}, err
	}

	return mapCustomerOrderSummary(summary), nil
}

func mapCustomerOrderSummary(summary domain.CustomerOrderSummary) inbound.CustomerOrderSummaryOutput {
	return inbound.CustomerOrderSummaryOutput{
		CustomerID:  summary.CustomerID,
		FullName:    summary.FullName,
		TotalOrders: summary.TotalOrders,
		TotalSpent:  summary.TotalSpent,
		TotalItems:  summary.TotalItems,
	}
}
