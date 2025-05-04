package models

import (
	"errors"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidOrderNumber = errors.New("invalid order number")
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

// ValidateOrderNumber проверяет номер заказа с помощью алгоритма Луна
func (o *Order) ValidateOrderNumber() bool {

	number := o.Number
	sum := 0
	parity := len(number) % 2

	for i, digitChar := range number {
		digit, err := strconv.Atoi(string(digitChar))
		if err != nil {
			return false
		}

		if i%2 == parity {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
	}

	return sum%10 == 0

}
