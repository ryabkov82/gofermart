package register

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/ryabkov82/gofermart/internal/app/handlers/users"
	"github.com/ryabkov82/gofermart/internal/app/models"
	"github.com/ryabkov82/gofermart/internal/app/storage"

	"go.uber.org/zap"
)

type URLHandler interface {
	RegisterUser(context.Context, string, string) (*models.User, error)
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

		user, err := urlHandler.RegisterUser(req.Context(), request.Login, request.Password)

		if err != nil {
			if errors.Is(err, storage.ErrLoginExists) {
				http.Error(res, "Login already exists", http.StatusConflict)
				log.Error("Login already exists", zap.String("login", request.Login), zap.Int("UserID", user.UserID))
				return
			}
			http.Error(res, "Internal server error", http.StatusInternalServerError)
			log.Error("Failed to register user", zap.Error(err))
			return
		}

		// После успешного создания пользователя генерируем JWT
		err = users.IssueNewToken(res, user.UserID, user.Login, jwtKey)
		if err != nil {
			http.Error(res, "Failed to generate token", http.StatusInternalServerError)
			log.Error("Failed to generate user", zap.Error(err))
			return
		}

		res.WriteHeader(http.StatusOK)
		json.NewEncoder(res).Encode(map[string]string{"status": "success"})
	}
}
