package inbound

import "context"

type CustomerOrderSummaryOutput struct {
	CustomerID  int64
	FullName    string
	TotalOrders int64
	TotalSpent  int64
	TotalItems  int64
}

type CustomerOrderSummaryUseCase interface {
	GetOrderSummary(ctx context.Context, customerID int64) (CustomerOrderSummaryOutput, error)
}
