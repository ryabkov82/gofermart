package login

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"go.uber.org/zap"

	"github.com/ryabkov82/gofermart/internal/app/handlers/users"
	"github.com/ryabkov82/gofermart/internal/app/models"
	"github.com/ryabkov82/gofermart/internal/app/storage"
)

type URLHandler interface {
	GetUserByLogin(context.Context, string) (*models.User, error)
}

func GetHandler(urlHandler URLHandler, jwtKey []byte, log *zap.Logger) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {

		// Парсинг JSON из тела запроса
		var request users.Request
		if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
			http.Error(res, "Invalid request format", http.StatusBadRequest)
			return
		}

		// Валидация данных
		if strings.TrimSpace(request.Login) == "" || strings.TrimSpace(request.Password) == "" {
			http.Error(res, "Login and password are required", http.StatusBadRequest)
			return
		}

		// Ищем пользователя в базе
		user, err := urlHandler.GetUserByLogin(req.Context(), request.Login)
		if err != nil {
			if errors.Is(err, storage.ErrLoginNotFound) {
				http.Error(res, "Login not found", http.StatusUnauthorized)
				log.Error("Failed to get user by login", zap.String("login", request.Login))
				return
			}
			http.Error(res, "Internal server error", http.StatusInternalServerError)
			log.Error("Failed to get user by login", zap.Error(err))
			return
		}

		// Проверяем пароль
		if !user.CheckPassword(request.Password) {
			http.Error(res, "Invalid login or password", http.StatusUnauthorized)
			log.Error("Invalid login or password", zap.String("login", request.Login))
			return
		}

		log.Info("Login successful", zap.String("login", request.Login))

		// Генерируем токены
		err = users.IssueNewToken(res, user.UserID, user.Login, jwtKey)
		if err != nil {
			http.Error(res, "Failed to generate token", http.StatusInternalServerError)
			log.Error("Failed to generate token", zap.Error(err))
			return
		}

		res.WriteHeader(http.StatusOK)
		json.NewEncoder(res).Encode(map[string]string{"status": "success"})

	}
}
