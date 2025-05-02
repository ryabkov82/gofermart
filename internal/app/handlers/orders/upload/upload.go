package upload

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"github.com/ryabkov82/gofermart/internal/app/models"
	"github.com/ryabkov82/gofermart/internal/app/storage"
	"github.com/ryabkov82/gofermart/internal/app/utils/jwtauth"
)

type URLHandler interface {
	AddOrder(context.Context, string) (*models.Order, error)
}

func GetHandler(urlHandler URLHandler, log *zap.Logger) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {

		// Проверка Content-Type
		if req.Header.Get("Content-Type") != "text/plain" {
			http.Error(res, "Invalid content type", http.StatusBadRequest)
			log.Error("Invalid content type")
			return
		}

		// Чтение номера заказа
		orderNumber, err := readOrderNumber(req)
		if err != nil {
			http.Error(res, err.Error(), http.StatusBadRequest)
			log.Error(err.Error())
			return
		}

		order, err := urlHandler.AddOrder(req.Context(), orderNumber)

		status := http.StatusAccepted

		if err != nil {
			log.Error(err.Error())
			status = http.StatusInternalServerError
			if errors.Is(err, models.ErrInvalidOrderNumber) {
				status = http.StatusUnprocessableEntity
			}
			if errors.Is(err, storage.ErrOrderExists) {
				userID := req.Context().Value(jwtauth.UserIDContextKey)
				if userID == order.UserID {
					status = http.StatusOK
				} else {
					status = http.StatusConflict
				}
			}
		}
		res.WriteHeader(status)
	}
}

func readOrderNumber(r *http.Request) (string, error) {
	buf := make([]byte, 1024)
	n, err := r.Body.Read(buf)
	if err != nil && err.Error() != "EOF" {
		return "", errors.New("failed to read request body")
	}

	orderNumber := strings.TrimSpace(string(buf[:n]))
	if _, err := strconv.Atoi(orderNumber); err != nil {
		return "", errors.New("order number must contain only digits")
	}

	return orderNumber, nil
}
