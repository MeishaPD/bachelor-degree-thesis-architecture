package postgres

import (
	"context"

	"thesis/hexagonal-backend/internal/core/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

type OrderRepository struct {
	db *pgxpool.Pool
}

func NewOrderRepository(db *pgxpool.Pool) *OrderRepository {
	return &OrderRepository{db: db}
}

func (r *OrderRepository) UpdateStatus(ctx context.Context, orderID int64, status string) error {
	const query = `
		UPDATE thesis_ecommerce.orders
		SET order_status = $1
		WHERE order_id = $2;	
	`

	result, err := r.db.Exec(ctx, query, status, orderID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}
