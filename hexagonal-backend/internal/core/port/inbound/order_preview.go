package inbound

type OrderItemInput struct {
	ProductID int64
	Quantity  int
}

type OrderPreviewInput struct {
	CustomerTier string
	Items        []OrderItemInput
}

type OrderPreviewItemOutput struct {
	ProductID int64
	Name      string
	Category  string
	Quantity  int
	UnitPrice int64
	Subtotal  int64
}

type OrderPreviewOutput struct {
	Items      []OrderPreviewItemOutput
	Subtotal   int64
	Discount   int64
	FinalTotal int64
}

type OrderPreviewUseCase interface {
	Execute(input OrderPreviewInput) (OrderPreviewOutput, error)
}
