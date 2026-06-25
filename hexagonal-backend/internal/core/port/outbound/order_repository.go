package outbound

import "context"

type OrderRepository interface {
	UpdateStatus(ctx context.Context, orderID int64, status string) error
}
