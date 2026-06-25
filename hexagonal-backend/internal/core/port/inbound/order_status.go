package inbound

import "context"

type UpdateOrderStatusInput struct {
	OrderID int64
	Status  string
}

type UpdateOrderStatusOutput struct {
	OrderID int64
	Status  string
}

type OrderStatusUseCase interface {
	UpdateStatus(ctx context.Context, input UpdateOrderStatusInput) (UpdateOrderStatusOutput, error)
}
