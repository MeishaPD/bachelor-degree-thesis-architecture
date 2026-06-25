package domain

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

func NewOrderStatus(value string) (OrderStatus, error) {
	status := OrderStatus(strings.ToLower(strings.TrimSpace(value)))

	switch status {
	case OrderStatusPending,
		OrderStatusPaid,
		OrderStatusShipped,
		OrderStatusCompleted,
		OrderStatusCancelled:
		return status, nil
	default:
		return "", errors.New("status pesanan tidak valid")
	}
}
