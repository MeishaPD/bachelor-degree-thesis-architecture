package postgres

import (
	"context"
	"errors"

	"thesis/layered-backend/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CustomerRepository struct {
	db *pgxpool.Pool
}

func NewCustomerRepository(db *pgxpool.Pool) *CustomerRepository {
	return &CustomerRepository{db: db}
}

func (r *CustomerRepository) GetOrderSummary(ctx context.Context, customerID int64) (model.CustomerOrderSummary, error) {
	const query = `
		SELECT c.customer_id,
			   c.full_name,
			   COUNT(DISTINCT o.order_id) AS total_orders,
			   COALESCE(SUM(o.total_amount), 0)::bigint AS total_spent,
			   COALESCE(SUM(oi.quantity), 0)::bigint AS total_items
		FROM thesis_ecommerce.customers c
		LEFT JOIN thesis_ecommerce.orders o ON c.customer_id = o.customer_id
		LEFT JOIN thesis_ecommerce.order_items oi ON o.order_id = oi.order_id
		WHERE c.customer_id = $1
		GROUP BY c.customer_id, c.full_name;
	`

	var summary model.CustomerOrderSummary
	err := r.db.QueryRow(ctx, query, customerID).Scan(
		&summary.CustomerID,
		&summary.FullName,
		&summary.TotalOrders,
		&summary.TotalSpent,
		&summary.TotalItems,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.CustomerOrderSummary{}, model.ErrNotFound
		}
		return model.CustomerOrderSummary{}, err
	}

	return summary, nil
}
