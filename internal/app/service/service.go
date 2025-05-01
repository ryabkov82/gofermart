package service

import (
	"context"

	"github.com/ryabkov82/gofermart/internal/app/models"
)

type Repository interface {
	CreateUser(context.Context, *models.User) error
	GetUserByLogin(context.Context, string) (*models.User, error)
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
