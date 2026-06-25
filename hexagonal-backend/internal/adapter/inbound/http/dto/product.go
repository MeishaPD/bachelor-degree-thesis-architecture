package dto

type ProductResponse struct {
	ProductID int64  `json:"product_id"`
	SKU       string `json:"sku"`
	Name      string `json:"product_name"`
	Price     int64  `json:"price"`
	StockQty  int    `json:"stock_qty"`
}
