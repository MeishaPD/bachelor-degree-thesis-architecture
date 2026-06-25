package domain

import "errors"

type OrderItem struct {
	Product  Product
	Quantity int
}

func NewOrderItem(product Product, quantity int) (OrderItem, error) {
	if quantity <= 0 {
		return OrderItem{}, errors.New("quantity harus lebih dari 0")
	}
	if !product.Active {
		return OrderItem{}, errors.New("produk tidak aktif")
	}
	return OrderItem{Product: product, Quantity: quantity}, nil
}

func (i OrderItem) Subtotal() int64 {
	return int64(i.Quantity) * i.Product.Price
}

type OrderPreviewItem struct {
	ProductID int64
	Name      string
	Category  string
	Quantity  int
	UnitPrice int64
	Subtotal  int64
}

type OrderPreviewResult struct {
	Items      []OrderPreviewItem
	Subtotal   int64
	Discount   int64
	FinalTotal int64
}
