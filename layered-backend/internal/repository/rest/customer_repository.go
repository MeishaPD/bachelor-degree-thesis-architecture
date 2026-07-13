package rest

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"thesis/layered-backend/internal/model"
)

type CustomerRepository struct {
	client *Client
}

type customerOrderSummaryResponse struct {
	CustomerID  int64  `json:"customer_id"`
	FullName    string `json:"full_name"`
	TotalOrders int64  `json:"total_orders"`
	TotalSpent  int64  `json:"total_spent"`
	TotalItems  int64  `json:"total_items"`
}

func NewCustomerRepository(client *Client) *CustomerRepository {
	return &CustomerRepository{client: client}
}

func (r *CustomerRepository) GetOrderSummary(ctx context.Context, customerID int64) (model.CustomerOrderSummary, error) {
	query := url.Values{}
	query.Set("select", "customer_id,full_name,total_orders,"+"total_spent,total_items")
	query.Set(
		"customer_id",
		"eq."+strconv.FormatInt(customerID, 10),
	)
	query.Set("limit", "1")

	var summaries []customerOrderSummaryResponse

	err := r.client.DoJSON(
		ctx,
		http.MethodGet,
		"customer_order_summaries",
		query,
		nil,
		nil,
		&summaries,
	)
	if err != nil {
		return model.CustomerOrderSummary{}, err
	}

	if len(summaries) == 0 {
		return model.CustomerOrderSummary{}, model.ErrNotFound
	}

	summary := summaries[0]

	return model.CustomerOrderSummary{
		CustomerID:  summary.CustomerID,
		FullName:    summary.FullName,
		TotalOrders: summary.TotalOrders,
		TotalSpent:  summary.TotalSpent,
		TotalItems:  summary.TotalItems,
	}, nil
}
