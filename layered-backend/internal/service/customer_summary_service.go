package service

import (
	"context"

	"thesis/layered-backend/internal/model"
)

type CustomerRepository interface {
	GetOrderSummary(ctx context.Context, customerID int64) (model.CustomerOrderSummary, error)
}

type CustomerOrderSummaryService struct {
	repository CustomerRepository
}

func NewCustomerOrderSummaryService(repository CustomerRepository) *CustomerOrderSummaryService {
	return &CustomerOrderSummaryService{repository: repository}
}

func (s *CustomerOrderSummaryService) GetOrderSummary(ctx context.Context, customerID int64) (model.CustomerOrderSummary, error) {
	summary, err := s.repository.GetOrderSummary(ctx, customerID)
	if err != nil {
		return model.CustomerOrderSummary{}, err
	}

	return summary, nil
}
