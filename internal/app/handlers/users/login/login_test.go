package login

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/ryabkov82/gofermart/internal/app/service"
	"github.com/ryabkov82/gofermart/internal/app/service/mocks"
	"golang.org/x/crypto/bcrypt"

	"github.com/ryabkov82/gofermart/internal/app/utils/jwtauth"
	"github.com/ryabkov82/gofermart/internal/app/utils/logger"

	"github.com/ryabkov82/gofermart/internal/app/models"
	mwlogger "github.com/ryabkov82/gofermart/internal/app/server/middleware/logger"
	"github.com/ryabkov82/gofermart/internal/app/server/middleware/mwgzip"

	"github.com/go-chi/chi/v5"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestGetHandler(t *testing.T) {

	// создаём контроллер
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// создаём объект-заглушку
	mockDB := mocks.NewMockRepository(ctrl)

	// инициализируем service объектом-заглушкой
	service := service.NewService(mockDB)

	if err := logger.Initialize("debug"); err != nil {
		panic(err)
	}
	var testSecretKey = []byte("test-secret-key")

	r := chi.NewRouter()
	r.Use(mwlogger.RequestLogging(logger.Log))
	r.Use(mwgzip.Gzip)
	r.Post("/api/user/login", GetHandler(service, testSecretKey, logger.Log))

	// запускаем тестовый сервер, будет выбран первый свободный порт
	srv := httptest.NewServer(r)
	// останавливаем сервер после завершения теста
	defer srv.Close()

	// Пользователь с хешированным паролем
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("correct_password"), bcrypt.DefaultCost)
	mockUser := &models.User{
		UserID:       1,
		Login:        "test_user",
		PasswordHash: string(hashedPassword),
	}

	tests := []struct {
		name           string
		payload        interface{}
		mockSetup      func() // Настройка мока
		expectedStatus int
		expectToken    bool
	}{
		{
			name:    "Успешная аутентификация",
			payload: map[string]string{"login": "test_user", "password": "correct_password"},
			mockSetup: func() {
				mockDB.EXPECT().
					GetUserByLogin(gomock.Any(), "test_user").
					Return(mockUser, nil)
			},
			expectedStatus: http.StatusOK,
			expectToken:    true,
		},
		{
			name:    "Неверный пароль",
			payload: map[string]string{"login": "test_user", "password": "wrong_password"},
			mockSetup: func() {
				mockDB.EXPECT().
					GetUserByLogin(gomock.Any(), "test_user").
					Return(mockUser, nil)
			},
			expectedStatus: http.StatusUnauthorized,
			expectToken:    false,
		},
		{
			name:           "Невалидный JSON",
			payload:        "invalid_json",
			mockSetup:      func() {}, // Мок не вызывается
			expectedStatus: http.StatusBadRequest,
			expectToken:    false,
		},
		{
			name:    "Ошибка БД",
			payload: map[string]string{"login": "test", "password": "test"},
			mockSetup: func() {
				mockDB.EXPECT().
					GetUserByLogin(gomock.Any(), "test").
					Return(nil, errors.New("db error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectToken:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Настраиваем мок
			tt.mockSetup()
			// Подготовка тела запроса
			body, _ := json.Marshal(tt.payload)

			buf := bytes.NewBuffer(nil)
			zb := gzip.NewWriter(buf)
			_, err := zb.Write(body)
			assert.NoError(t, err)
			err = zb.Close()
			assert.NoError(t, err)

			resp, err := resty.New().R().
				SetBody(buf).
				SetHeader("Content-Encoding", "gzip").
				SetHeader("Accept-Encoding", "gzip").
				Post(srv.URL + "/api/user/login")

			assert.NoError(t, err)

			// Проверяем статус
			assert.Equal(t, tt.expectedStatus, resp.StatusCode())

			// Проверка куки
			if tt.expectToken {
				cookie := resp.Cookies()[0]
				assert.Equal(t, "token", cookie.Name)
				assert.NotEmpty(t, cookie.Value)

				// Проверка валидности JWT
				tokenStr := cookie.Value
				claims := &jwtauth.Claims{}

				token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
					if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
						return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
					}
					return testSecretKey, nil
				})

				assert.NoError(t, err)
				assert.True(t, token.Valid)

				assert.Equal(t, int(1), claims.UserID)
				assert.Equal(t, "test_user", claims.Login)
			} else {
				assert.Empty(t, resp.Cookies())
			}
		})
	}
}
