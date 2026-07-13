package service

import (
	"context"
	"errors"

	"thesis/layered-backend/internal/model"
)

type OrderRepository interface {
	UpdateStatus(ctx context.Context, orderID int64, status string) error
}

type OrderStatusService struct {
	repository OrderRepository
}

func NewOrderStatusService(repository OrderRepository) *OrderStatusService {
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
