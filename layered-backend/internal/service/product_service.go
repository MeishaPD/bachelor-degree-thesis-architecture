package service

import (
	"context"
	"thesis/layered-backend/internal/model"
)

type ProductRepository interface {
	FindByID(ctx context.Context, id int64) (model.ProductDetail, error)
}

type ProductService struct {
	repository ProductRepository
}

func NewProductService(repository ProductRepository) *ProductService {
	return &ProductService{repository: repository}
}

func (s *ProductService) GetByID(ctx context.Context, id int64) (model.ProductDetail, error) {
	product, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return model.ProductDetail{}, err
	}

	return product, nil
}
