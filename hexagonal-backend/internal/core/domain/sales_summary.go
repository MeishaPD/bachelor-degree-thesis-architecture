package domain

type SalesSummary struct {
	StartDate         string
	EndDate           string
	TotalOrders       int64
	GrossRevenue      int64
	TotalItems        int64
	AverageOrderValue int64
}
