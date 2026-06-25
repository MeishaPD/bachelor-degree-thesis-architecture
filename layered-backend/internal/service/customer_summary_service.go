package service

import (
	"context"

	"thesis/layered-backend/internal/model"
	"thesis/layered-backend/internal/repository/postgres"
)

type CustomerOrderSummaryService struct {
	repository *postgres.CustomerRepository
}

func NewCustomerOrderSummaryService(repository *postgres.CustomerRepository) *CustomerOrderSummaryService {
	return &CustomerOrderSummaryService{repository: repository}
}

func (s *CustomerOrderSummaryService) GetOrderSummary(ctx context.Context, customerID int64) (model.CustomerOrderSummary, error) {
	summary, err := s.repository.GetOrderSummary(ctx, customerID)
	if err != nil {
		return model.CustomerOrderSummary{}, err
	}

	return summary, nil
}
