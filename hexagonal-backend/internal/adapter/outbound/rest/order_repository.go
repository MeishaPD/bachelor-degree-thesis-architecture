package rest

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"thesis/hexagonal-backend/internal/core/domain"
)

type OrderRepository struct {
	client *Client
}

type updatedOrderResponse struct {
	OrderID int64 `json:"order_id"`
}

func NewOrderRepository(client *Client) *OrderRepository {
	return &OrderRepository{client: client}
}

func (r *OrderRepository) UpdateStatus(ctx context.Context, orderID int64, status string) error {
	query := url.Values{}
	query.Set(
		"order_id",
		"eq."+strconv.FormatInt(orderID, 10),
	)
	query.Set("select", "order_id")

	payload := map[string]string{
		"order_status": status,
	}

	headers := map[string]string{
		"Prefer": "return=representation",
	}

	var updatedOrders []updatedOrderResponse

	err := r.client.DoJSON(
		ctx,
		http.MethodPatch,
		"orders",
		query,
		payload,
		headers,
		&updatedOrders,
	)
	if err != nil {
		return err
	}

	if len(updatedOrders) == 0 {
		return domain.ErrNotFound
	}

	return nil
}
