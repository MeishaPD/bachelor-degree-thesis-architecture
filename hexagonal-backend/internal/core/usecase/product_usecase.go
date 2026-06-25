package usecase

import (
	"context"

	"thesis/hexagonal-backend/internal/core/domain"
	"thesis/hexagonal-backend/internal/core/port/inbound"
	"thesis/hexagonal-backend/internal/core/port/outbound"
)

type ProductUseCase struct {
	repository outbound.ProductRepository
}

func NewProductUseCase(repository outbound.ProductRepository) *ProductUseCase {
	return &ProductUseCase{repository: repository}
}

func (u *ProductUseCase) GetByID(ctx context.Context, id int64) (inbound.ProductOutput, error) {
	product, err := u.repository.FindByID(ctx, id)
	if err != nil {
		return inbound.ProductOutput{}, err
	}

	return mapProductDetail(product), nil
}

func mapProductDetail(product domain.ProductDetail) inbound.ProductOutput {
	return inbound.ProductOutput{
		ProductID: product.ID,
		SKU:       product.SKU,
		Name:      product.Name,
		Price:     product.Price,
		StockQty:  product.StockQty,
	}
}
