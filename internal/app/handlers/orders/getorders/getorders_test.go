package getorders

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/ryabkov82/gofermart/internal/app/service"
	"github.com/ryabkov82/gofermart/internal/app/service/mocks"

	"github.com/ryabkov82/gofermart/internal/app/models"
	"github.com/ryabkov82/gofermart/internal/app/utils/jwtauth"
	"github.com/ryabkov82/gofermart/internal/app/utils/logger"

	"github.com/ryabkov82/gofermart/internal/app/server/middleware/auth"
	mwlogger "github.com/ryabkov82/gofermart/internal/app/server/middleware/logger"
	"github.com/ryabkov82/gofermart/internal/app/server/middleware/mwgzip"

	"github.com/go-chi/chi/v5"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
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
	r.Get("/api/user/orders", GetHandler(service, logger.Log))

	// запускаем тестовый сервер, будет выбран первый свободный порт
	srv := httptest.NewServer(r)
	// останавливаем сервер после завершения теста
	defer srv.Close()

	cookie1 := createSignedCookie(1, "test_user1")

	tests := []struct {
		name           string
		setupMocks     func() // Настройка мока
		cookie         *http.Cookie
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "successful request with orders",
			setupMocks: func() {
				mockDB.EXPECT().
					GetUserOrders(gomock.Any(), gomock.Any()).Return([]models.Order{
					{
						Number:     "9278923470",
						Status:     "PROCESSED",
						Accrual:    500,
						UploadedAt: time.Date(2020, 12, 10, 15, 15, 45, 0, time.FixedZone("", 3*60*60)),
					},
				}, nil)
			},
			cookie:         cookie1,
			expectedStatus: http.StatusOK,
			expectedBody:   `[{"number":"9278923470","status":"PROCESSED","accrual":500,"uploaded_at":"2020-12-10T15:15:45+03:00"}]`,
		},
		{
			name: "no orders found",
			setupMocks: func() {
				mockDB.EXPECT().
					GetUserOrders(gomock.Any(), gomock.Any()).Return([]models.Order{}, nil)
			},
			cookie:         cookie1,
			expectedStatus: http.StatusNoContent,
			expectedBody:   "",
		},
		{
			name:           "no token cookie",
			setupMocks:     func() {},
			cookie:         &http.Cookie{Name: "session", Value: ""},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "",
		},
		{
			name:           "invalid token",
			setupMocks:     func() {},
			cookie:         &http.Cookie{Name: "token", Value: "invalid_token"},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "",
		},
		{
			name: "database error",
			setupMocks: func() {
				mockDB.EXPECT().
					GetUserOrders(gomock.Any(), gomock.Any()).Return(nil, errors.New("db error"))
			},
			cookie:         cookie1,
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			tt.setupMocks()

			resp, err := resty.New().R().
				SetCookie(tt.cookie).
				SetHeader("Accept-Encoding", "gzip").
				Get(srv.URL + "/api/user/orders")

			assert.NoError(t, err)

			assert.Equal(t, tt.expectedStatus, resp.StatusCode())

			if tt.expectedBody != "" {
				var expected, actual interface{}
				err = json.Unmarshal([]byte(tt.expectedBody), &expected)
				assert.NoError(t, err)
				err = json.Unmarshal(resp.Body(), &actual)
				assert.NoError(t, err)
				assert.Equal(t, expected, actual)
			}
		})
	}
}
