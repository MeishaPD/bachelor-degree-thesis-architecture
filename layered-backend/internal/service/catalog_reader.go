package service

import "thesis/layered-backend/internal/model"

type CatalogReader interface {
	GetProductByID(id int64) (model.Product, bool)
}
