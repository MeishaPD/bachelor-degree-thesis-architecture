package model

type CustomerOrderSummary struct {
	CustomerID  int64
	FullName    string
	TotalOrders int64
	TotalSpent  int64
	TotalItems  int64
}
