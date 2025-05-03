package accrual

import (
	"context"
	"time"

	"github.com/ryabkov82/gofermart/internal/app/models"
)

type AccrualRepository interface {
	ProcessAccruals(context.Context, map[int]float64, []models.Order) error
	GetOrdersForProcessing(context.Context, int, []string) ([]models.Order, error)
}

type AccrualService struct {
	repo   AccrualRepository
	client AccrualClient
}

func NewService(storage AccrualRepository, client AccrualClient) *AccrualService {
	return &AccrualService{
		repo:   storage,
		client: client,
	}
}

func (s *AccrualService) ProcessBatchAccruals(userAccruals map[int]float64, orderUpdates []models.Order) error {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := s.repo.ProcessAccruals(ctx, userAccruals, orderUpdates)

	return err

}

func (s *AccrualService) GetOrderInfo(ctx context.Context, orderNumber string) (*models.OrderAccrual, error) {

	// Устанавливаем таймаут для отдельного запроса
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	accrualInfo, err := s.client.GetOrderInfo(reqCtx, orderNumber)

	return accrualInfo, err

}

func (s *AccrualService) GetOrdersForProcessing(ctx context.Context, limit int, activeTasks []string) ([]models.Order, error) {

	orders, err := s.repo.GetOrdersForProcessing(ctx, limit, activeTasks)

	return orders, err
}
