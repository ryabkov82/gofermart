package upload

import (
	"bytes"
	"compress/gzip"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/ryabkov82/gofermart/internal/app/service"
	"github.com/ryabkov82/gofermart/internal/app/service/mocks"
	"golang.org/x/net/context"

	"github.com/ryabkov82/gofermart/internal/app/models"
	"github.com/ryabkov82/gofermart/internal/app/storage"
	"github.com/ryabkov82/gofermart/internal/app/utils/jwtauth"
	"github.com/ryabkov82/gofermart/internal/app/utils/logger"

	"github.com/ryabkov82/gofermart/internal/app/server/middleware/auth"
	mwlogger "github.com/ryabkov82/gofermart/internal/app/server/middleware/logger"
	"github.com/ryabkov82/gofermart/internal/app/server/middleware/mwgzip"

	"github.com/go-chi/chi/v5"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

const (
	validOrder   = "12345678903"
	invalidOrder = "12345678901"
)

var (
	testSecretKey = []byte("test-secret-key")
)

func createSignedCookie(userID int, login string) *http.Cookie {

	tokenString, err := jwtauth.GenerateNewToken(userID, login, testSecretKey)
	if err != nil {
		panic(err)
	}

	return &http.Cookie{
		Name:     "token",
		Value:    tokenString,
		HttpOnly: true,
		Path:     "/",
		SameSite: http.SameSiteStrictMode,
	}

}

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

	r := chi.NewRouter()
	r.Use(mwlogger.RequestLogging(logger.Log))
	r.Use(mwgzip.Gzip)
	r.Use(auth.AuthMiddleware(testSecretKey))
	r.Post("/api/user/orders", GetHandler(service, logger.Log))

	// запускаем тестовый сервер, будет выбран первый свободный порт
	srv := httptest.NewServer(r)
	// останавливаем сервер после завершения теста
	defer srv.Close()

	cookie1 := createSignedCookie(1, "test_user1")

	tests := []struct {
		name        string
		setupMock   func() // Настройка мока
		cookie      *http.Cookie
		requestBody string
		wantStatus  int
	}{
		// Успешные сценарии
		{
			name: "202 Accepted - new order",
			setupMock: func() {
				mockDB.EXPECT().
					AddOrder(gomock.Any(), gomock.Any()).
					Return(nil)
			},
			cookie:      cookie1,
			requestBody: validOrder,
			wantStatus:  http.StatusAccepted,
		},
		{
			name: "200 OK - order exists for user",
			setupMock: func() {
				mockDB.EXPECT().
					AddOrder(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, order *models.Order) error {
						order.UserID = 1 // Имитируем присвоение ID
						return storage.ErrOrderExists
					})
			},
			cookie:      cookie1,
			requestBody: validOrder,
			wantStatus:  http.StatusOK,
		},

		// Ошибки аутентификации
		{
			name:        "401 Unauthorized - no token",
			setupMock:   func() {},
			cookie:      &http.Cookie{Name: "session", Value: ""},
			requestBody: validOrder,
			wantStatus:  http.StatusUnauthorized,
		},
		{
			name:        "422 Unprocessable Entity - invalid Luhn check",
			setupMock:   func() {},
			requestBody: invalidOrder,
			cookie:      cookie1,
			wantStatus:  http.StatusUnprocessableEntity,
		},

		// Конфликты
		{
			name: "409 Conflict - order exists for other user",
			setupMock: func() {
				mockDB.EXPECT().
					AddOrder(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, order *models.Order) error {
						order.UserID = 2 // Имитируем присвоение ID
						return storage.ErrOrderExists
					})
			},
			cookie:      cookie1,
			requestBody: validOrder,
			wantStatus:  http.StatusConflict,
		},

		// Ошибки сервера
		{
			name: "500 Internal Server Error - db error",
			setupMock: func() {
				mockDB.EXPECT().
					AddOrder(gomock.Any(), gomock.Any()).
					Return(errors.New("database error"))
			},
			requestBody: validOrder,
			cookie:      cookie1,
			wantStatus:  http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Настраиваем мок
			tt.setupMock()
			// Подготовка тела запроса

			buf := bytes.NewBuffer(nil)
			zb := gzip.NewWriter(buf)
			_, err := zb.Write([]byte(tt.requestBody))
			assert.NoError(t, err)
			err = zb.Close()
			assert.NoError(t, err)

			resp, err := resty.New().R().
				SetBody(buf).
				SetCookie(tt.cookie).
				SetHeader("Content-Encoding", "gzip").
				SetHeader("Accept-Encoding", "gzip").
				SetHeader("Content-Type", "text/plain").
				Post(srv.URL + "/api/user/orders")

			assert.NoError(t, err)

			// Проверяем статус
			assert.Equal(t, tt.wantStatus, resp.StatusCode())

		})
	}

}
