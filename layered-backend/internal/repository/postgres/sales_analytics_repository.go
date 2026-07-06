package postgres

import (
	"context"
	"thesis/layered-backend/internal/model"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SalesAnalyticsRepository struct {
	db *pgxpool.Pool
}

func NewSalesAnalyticsRepository(db *pgxpool.Pool) *SalesAnalyticsRepository {
	return &SalesAnalyticsRepository{db: db}
}

func (r *SalesAnalyticsRepository) GetSalesSummary(ctx context.Context, startDate time.Time, endDate time.Time) (model.SalesSummary, error) {
	const query = `
		WITH filtered_orders AS (
			SELECT
				order_id,
				total_amoun
			FROM thesis_ecommerce.orders
			WHERE order_date >= $1::date
				AND order_date < ($2::date + INTERVAL '1 day')
				AND order_status IN ('paid', 'shipped', 'completed')
		),
		item_summary AS (
			SELECT
				oi.order_id,
				SUM(oi.quantity)::bigint AS total_items
			FROM thesis_ecommerce.order_items oi
			JOIN filtered_orders fo ON fo.order_id = oi.order_id
			GROUP BY oi.order_id
		)
		SELECT
			COUNT(fo.order_id)::bigint AS total_orders,
			COALESCE(SUM(fo.total_amount), 0)::bigint AS gross_revenue,
			COALESCE(SUM(item_summary.total_items), 0)::bigint AS total_items,
			CASE
				WHEN COUNT(fo.order_id) = 0 THEN 0
				ELSE ROUND(COALESCE(SUM(fo.total_amount), 0) / COUNT(fo.order_id))::bigint
			END AS average_order_value
		FROM filtered_orders fo
		LEFT JOIN item_summary ON item_summary.order_id = fo.order_id;
	`

	var summary model.SalesSummary

	err := r.db.QueryRow(ctx, query, startDate, endDate).Scan(
		&summary.TotalOrders,
		&summary.GrossRevenue,
		&summary.TotalItems,
		&summary.AverageOrderValue,
	)
	if err != nil {
		return model.SalesSummary{}, err
	}
	return summary, nil
}
