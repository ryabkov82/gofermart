package models

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	UserID       int    `json:"id"`
	Login        string `json:"login"`
	PasswordHash string `json:"-"` // Пропускаем в JSON
}

// OrderStatus представляет возможные статусы заказа
type OrderStatus string

const (
	OrderStatusNew        OrderStatus = "NEW"
	OrderStatusProcessing OrderStatus = "PROCESSING"
	OrderStatusProcessed  OrderStatus = "PROCESSED"
	OrderStatusInvalid    OrderStatus = "INVALID"
	OrderStatusRegistered OrderStatus = "REGISTERED"
)

type Balance struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

// Order представляет модель данных заказа
type Order struct {
	Number     string      `json:"number"`
	UserID     int         `json:"-"`
	Status     OrderStatus `json:"status"`
	Accrual    float64     `json:"accrual,omitempty"`
	UploadedAt time.Time   `json:"uploaded_at"`
}

// HashPassword создает bcrypt-хеш пароля
func (u *User) HashPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return nil
}

// CheckPassword проверяет пароль
func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	return err == nil
}
