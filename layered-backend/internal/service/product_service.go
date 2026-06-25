package service

import (
	"context"
	"thesis/layered-backend/internal/model"
	"thesis/layered-backend/internal/repository/postgres"
)

type ProductService struct {
	repository *postgres.ProductRepository
}

func NewProductService(repository *postgres.ProductRepository) *ProductService {
	return &ProductService{repository: repository}
}

func (s *ProductService) GetByID(ctx context.Context, id int64) (model.ProductDetail, error) {
	product, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return model.ProductDetail{}, err
	}

	return product, nil
}
