package inbound

import "context"

type ProductOutput struct {
	ProductID int64
	SKU       string
	Name      string
	Price     int64
	StockQty  int
}

type ProductUseCase interface {
	GetByID(ctx context.Context, id int64) (ProductOutput, error)
}
