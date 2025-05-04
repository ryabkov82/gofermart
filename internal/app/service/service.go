package service

import (
	"context"

	"github.com/ryabkov82/gofermart/internal/app/models"
	"github.com/ryabkov82/gofermart/internal/app/utils/jwtauth"
)

type Repository interface {
	CreateUser(context.Context, *models.User) error
	GetUserByLogin(context.Context, string) (*models.User, error)
	AddOrder(context.Context, *models.Order) error
	GetUserOrders(context.Context, int) ([]models.Order, error)
}

type Service struct {
	repo Repository
}

func NewService(storage Repository) *Service {

	return &Service{
		repo: storage,
	}
}

func (s *Service) RegisterUser(ctx context.Context, login string, password string) (*models.User, error) {

	// Создание пользователя в базе данных
	userDB := &models.User{
		Login: login,
	}

	err := userDB.HashPassword(password)
	if err != nil {
		return userDB, err
	}

	err = s.repo.CreateUser(ctx, userDB)

	return userDB, err
}

func (s *Service) GetUserByLogin(ctx context.Context, login string) (*models.User, error) {
	userDB, err := s.repo.GetUserByLogin(ctx, login)
	return userDB, err
}

func (s *Service) AddOrder(ctx context.Context, number string) (*models.Order, error) {

	userID := ctx.Value(jwtauth.UserIDContextKey)
	order := &models.Order{
		Number: number,
		UserID: userID.(int),
	}

	numberIsValid := order.ValidateOrderNumber()

	if !numberIsValid {
		return order, models.ErrInvalidOrderNumber
	}

	err := s.repo.AddOrder(ctx, order)

	return order, err

}

func (s *Service) GetUserOrders(ctx context.Context) ([]models.Order, error) {

	userID := ctx.Value(jwtauth.UserIDContextKey)
	orders, err := s.repo.GetUserOrders(ctx, userID.(int))
	return orders, err
}
