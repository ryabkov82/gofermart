package withdraw

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/ryabkov82/gofermart/internal/app/service"
	"github.com/ryabkov82/gofermart/internal/app/service/mocks"
	"github.com/stretchr/testify/assert"

	"github.com/ryabkov82/gofermart/internal/app/utils/jwtauth"
	"github.com/ryabkov82/gofermart/internal/app/utils/logger"

	"github.com/ryabkov82/gofermart/internal/app/server/middleware/auth"
	mwlogger "github.com/ryabkov82/gofermart/internal/app/server/middleware/logger"
	"github.com/ryabkov82/gofermart/internal/app/server/middleware/mwgzip"

	"github.com/go-chi/chi/v5"
	"github.com/golang/mock/gomock"
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
	srv := service.NewService(mockDB)

	if err := logger.Initialize("debug"); err != nil {
		panic(err)
	}

	r := chi.NewRouter()
	r.Use(mwlogger.RequestLogging(logger.Log))
	r.Use(mwgzip.Gzip)
	r.Use(auth.AuthMiddleware(testSecretKey))
	r.Post("/api/user/balance/withdraw", GetHandler(srv, logger.Log))

	// запускаем тестовый сервер, будет выбран первый свободный порт
	server := httptest.NewServer(r)
	// останавливаем сервер после завершения теста
	defer server.Close()

	cookie1 := createSignedCookie(1, "test_user1")

	tests := []struct {
		name           string
		cookie         *http.Cookie
		requestBody    WithdrawRequest
		setupMocks     func()
		expectedStatus int
	}{
		{
			name: "successful withdrawal",
			requestBody: WithdrawRequest{
				Order: "2377225624", // Valid Luhn number
				Sum:   100,
			},
			cookie: cookie1,
			setupMocks: func() {
				mockDB.EXPECT().WithdrawFunds(gomock.Any(), 1, "2377225624", 100.0).Return(nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "insufficient funds",
			requestBody: WithdrawRequest{
				Order: "2377225624",
				Sum:   600,
			},
			cookie: cookie1,
			setupMocks: func() {
				mockDB.EXPECT().WithdrawFunds(gomock.Any(), 1, "2377225624", 600.0).Return(service.ErrInsufficientFunds)
			},
			expectedStatus: http.StatusPaymentRequired,
		},
		{
			name: "invalid order number",
			requestBody: WithdrawRequest{
				Order: "1234567890", // Invalid Luhn number
				Sum:   100,
			},
			cookie:         cookie1,
			setupMocks:     func() {},
			expectedStatus: http.StatusUnprocessableEntity,
		},
		{
			name:           "no token cookie",
			setupMocks:     func() {},
			cookie:         &http.Cookie{Name: "session", Value: ""},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "database error",
			requestBody: WithdrawRequest{
				Order: "2377225624",
				Sum:   600,
			},
			setupMocks: func() {
				mockDB.EXPECT().WithdrawFunds(gomock.Any(), 1, "2377225624", 600.0).Return(errors.New("database error"))
			},
			cookie:         cookie1,
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			tt.setupMocks()

			// Подготовка тела запроса
			body, _ := json.Marshal(tt.requestBody)

			buf := bytes.NewBuffer(nil)
			zb := gzip.NewWriter(buf)
			_, err := zb.Write(body)
			assert.NoError(t, err)
			err = zb.Close()
			assert.NoError(t, err)

			resp, err := resty.New().R().
				SetBody(buf).
				SetCookie(tt.cookie).
				SetHeader("Content-Encoding", "gzip").
				SetHeader("Accept-Encoding", "gzip").
				Post(server.URL + "/api/user/balance/withdraw")
			assert.NoError(t, err)

			assert.Equal(t, tt.expectedStatus, resp.StatusCode())

		})
	}

}
