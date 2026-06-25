package model

import (
	"errors"
	"strings"
)

type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusPaid      OrderStatus = "paid"
	OrderStatusShipped   OrderStatus = "shipped"
	OrderStatusCompleted OrderStatus = "completed"
	OrderStatusCancelled OrderStatus = "cancelled"
)

func NewOrderStatus(status string) (OrderStatus, error) {
	s := OrderStatus(strings.ToLower(strings.TrimSpace(status)))

	switch s {
	case OrderStatusPending, OrderStatusPaid, OrderStatusShipped, OrderStatusCompleted, OrderStatusCancelled:
		return s, nil
	default:
		return "", errors.New("status pesanan tidak valid")
	}
}

type OrderStatusResult struct {
	OrderID int64
	Status  string
}
