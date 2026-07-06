package service

import (
	"context"
	"errors"
	"thesis/layered-backend/internal/model"
	"thesis/layered-backend/internal/repository/postgres"
	"time"
)

type SalesAnalyticsService struct {
	repository *postgres.SalesAnalyticsRepository
}

func NewSalesAnalyticsService(repository *postgres.SalesAnalyticsRepository) *SalesAnalyticsService {
	return &SalesAnalyticsService{repository: repository}
}

func (s *SalesAnalyticsService) GetSalesSummary(ctx context.Context, startDate time.Time, endDate time.Time) (model.SalesSummary, error) {
	if endDate.Before(startDate) {
		return model.SalesSummary{}, errors.New("end_date tidak boleh sebelum start_date")
	}

	summary, err := s.repository.GetSalesSummary(ctx, startDate, endDate)
	if err != nil {
		return model.SalesSummary{}, err
	}

	summary.StartDate = startDate.Format("2006-01-02")
	summary.EndDate = endDate.Format("2006-01-02")

	return summary, nil
}
