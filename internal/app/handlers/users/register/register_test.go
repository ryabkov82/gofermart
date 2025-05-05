package register

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
	"golang.org/x/net/context"

	"github.com/ryabkov82/gofermart/internal/app/models"
	"github.com/ryabkov82/gofermart/internal/app/storage"
	"github.com/ryabkov82/gofermart/internal/app/utils/jwtauth"
	"github.com/ryabkov82/gofermart/internal/app/utils/logger"

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
	r.Post("/api/user/register", GetHandler(service, testSecretKey, logger.Log))

	// запускаем тестовый сервер, будет выбран первый свободный порт
	srv := httptest.NewServer(r)
	// останавливаем сервер после завершения теста
	defer srv.Close()

	tests := []struct {
		name           string
		payload        map[string]string
		mockSetup      func()
		expectedStatus int
		checkToken     bool
	}{
		{
			name:    "Успешная регистрация с проверкой токена",
			payload: map[string]string{"login": "new_user", "password": "secure_password"},
			mockSetup: func() {
				mockDB.EXPECT().
					CreateUser(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, user *models.User) error {
						user.UserID = 1 // Имитируем присвоение ID
						return nil
					})
			},
			expectedStatus: http.StatusOK,
			checkToken:     true,
		},
		{
			name:    "Логин занят",
			payload: map[string]string{"login": "existing_user", "password": "12345"},
			mockSetup: func() {
				mockDB.EXPECT().
					CreateUser(gomock.Any(), gomock.Any()).
					Return(storage.ErrLoginExists)
			},
			expectedStatus: http.StatusConflict,
			checkToken:     false,
		},
		{
			name:           "Неверный формат запроса",
			payload:        map[string]string{},
			mockSetup:      func() {},
			expectedStatus: http.StatusBadRequest,
			checkToken:     false,
		},
		{
			name:    "Ошибка БД",
			payload: map[string]string{"login": "test", "password": "test"},
			mockSetup: func() {
				mockDB.EXPECT().
					CreateUser(gomock.Any(), gomock.Any()).
					Return(errors.New("db error"))
			},
			expectedStatus: http.StatusInternalServerError,
			checkToken:     false,
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
				Post(srv.URL + "/api/user/register")

			assert.NoError(t, err)

			// Проверяем статус
			assert.Equal(t, tt.expectedStatus, resp.StatusCode())

			// Проверка куки
			if tt.checkToken {
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
				assert.Equal(t, "new_user", claims.Login)
			} else {
				assert.Empty(t, resp.Cookies())
			}
		})
	}
}
