package usecase

import (
	"context"
	"errors"

	"thesis/hexagonal-backend/internal/core/domain"
	"thesis/hexagonal-backend/internal/core/port/inbound"
	"thesis/hexagonal-backend/internal/core/port/outbound"
)

type OrderStatusUseCase struct {
	repository outbound.OrderRepository
}

func NewOrderStatusUseCase(repository outbound.OrderRepository) *OrderStatusUseCase {
	return &OrderStatusUseCase{repository: repository}
}

func (u *OrderStatusUseCase) UpdateStatus(ctx context.Context, req inbound.UpdateOrderStatusInput) (inbound.UpdateOrderStatusOutput, error) {
	if req.OrderID <= 0 {
		return inbound.UpdateOrderStatusOutput{}, errors.New("order id tidak valid")
	}

	status, err := domain.NewOrderStatus(req.Status)
	if err != nil {
		return inbound.UpdateOrderStatusOutput{}, err
	}

	if err := u.repository.UpdateStatus(ctx, req.OrderID, string(status)); err != nil {
		return inbound.UpdateOrderStatusOutput{}, err
	}

	return inbound.UpdateOrderStatusOutput{
		OrderID: req.OrderID,
		Status:  string(status),
	}, nil
}
