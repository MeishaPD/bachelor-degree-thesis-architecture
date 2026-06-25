package model

type Product struct {
	ID       int64
	Name     string
	Category string
	Price    int64
	Active   bool
}

type ProductDetail struct {
	ID       int64
	SKU      string
	Name     string
	Price    int64
	StockQty int
}
