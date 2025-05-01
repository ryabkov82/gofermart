package models

import "golang.org/x/crypto/bcrypt"

type User struct {
	UserID       int    `json:"id"`
	Login        string `json:"login"`
	PasswordHash string `json:"-"` // Пропускаем в JSON
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
