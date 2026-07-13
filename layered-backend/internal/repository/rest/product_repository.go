package rest

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"thesis/layered-backend/internal/model"
)

type ProductRepository struct {
	client *Client
}

type productResponse struct {
	ProductID int64  `json:"product_id"`
	SKU       string `json:"sku"`
	Name      string `json:"product_name"`
	Price     int64  `json:"price"`
	StockQty  int    `json:"stock_qty"`
}

func NewProductRepository(client *Client) *ProductRepository {
	return &ProductRepository{client: client}
}

func (r *ProductRepository) FindByID(ctx context.Context, id int64) (model.ProductDetail, error) {
	query := url.Values{}
	query.Set("select", "product_id,sku,product_name,price,stock_qty")
	query.Set(
		"product_id",
		"eq."+strconv.FormatInt(id, 10),
	)
	query.Set("limit", "1")

	var products []productResponse

	err := r.client.DoJSON(
		ctx,
		http.MethodGet,
		"product_details",
		query,
		nil,
		nil,
		&products,
	)
	if err != nil {
		return model.ProductDetail{}, err
	}

	if len(products) == 0 {
		return model.ProductDetail{}, model.ErrNotFound
	}

	product := products[0]

	return model.ProductDetail{
		ID:       product.ProductID,
		SKU:      product.SKU,
		Name:     product.Name,
		Price:    product.Price,
		StockQty: product.StockQty,
	}, nil
}
