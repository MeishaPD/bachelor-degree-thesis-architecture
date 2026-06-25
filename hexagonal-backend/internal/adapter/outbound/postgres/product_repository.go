package postgres

import (
	"context"
	"errors"
	"thesis/hexagonal-backend/internal/core/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProductRepository struct {
	db *pgxpool.Pool
}

func NewProductRepository(db *pgxpool.Pool) *ProductRepository {
	return &ProductRepository{db: db}
}

func (r *ProductRepository) FindByID(ctx context.Context, id int64) (domain.ProductDetail, error) {
	const query = `
		SELECT	product_id,
				sku,
				product_name,
				price::bigint,
				stock_qty
		FROM thesis_ecommerce.products
		WHERE product_id = $1;
	`

	var product domain.ProductDetail
	err := r.db.QueryRow(ctx, query, id).Scan(
		&product.ID,
		&product.SKU,
		&product.Name,
		&product.Price,
		&product.StockQty,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ProductDetail{}, domain.ErrNotFound
		}
		return domain.ProductDetail{}, err
	}

	return product, nil
}
