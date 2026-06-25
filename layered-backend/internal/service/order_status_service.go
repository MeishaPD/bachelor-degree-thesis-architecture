package service

import (
	"context"
	"errors"

	"thesis/layered-backend/internal/model"
	"thesis/layered-backend/internal/repository/postgres"
)

type OrderStatusService struct {
	repository *postgres.OrderRepository
}

func NewOrderStatusService(repository *postgres.OrderRepository) *OrderStatusService {
	return &OrderStatusService{repository: repository}
}

func (s *OrderStatusService) UpdateStatus(ctx context.Context, orderID int64, value string) (model.OrderStatusResult, error) {
	if orderID <= 0 {
		return model.OrderStatusResult{}, errors.New("order id tidak valid")
	}

	status, err := model.NewOrderStatus(value)
	if err != nil {
		return model.OrderStatusResult{}, err
	}

	if err := s.repository.UpdateStatus(ctx, orderID, string(status)); err != nil {
		return model.OrderStatusResult{}, err
	}

	return model.OrderStatusResult{
		OrderID: orderID,
		Status:  string(status),
	}, nil
}
