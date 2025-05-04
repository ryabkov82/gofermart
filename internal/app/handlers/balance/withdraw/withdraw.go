package withdraw

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/ryabkov82/gofermart/internal/app/service"
	"go.uber.org/zap"
)

type WithdrawRequest struct {
	Order string  `json:"order"`
	Sum   float64 `json:"sum"`
}

type URLHandler interface {
	WithdrawFunds(context.Context, string, float64) error
}

func GetHandler(urlHandler URLHandler, log *zap.Logger) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {

		// Парсинг запроса
		var request WithdrawRequest
		if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
			http.Error(res, "Bad Request", http.StatusBadRequest)
			return
		}

		// Проверка суммы
		if request.Sum <= 0 {
			http.Error(res, "Invalid sum", http.StatusBadRequest)
			return
		}

		err := urlHandler.WithdrawFunds(req.Context(), request.Order, request.Sum)

		status := http.StatusOK

		if err != nil {
			log.Error(err.Error())
			status = http.StatusInternalServerError
			if errors.Is(err, service.ErrInvalidOrderNumber) {
				status = http.StatusUnprocessableEntity
			}
			if errors.Is(err, service.ErrInsufficientFunds) {
				status = http.StatusPaymentRequired
			}
		}
		res.WriteHeader(status)
	}
}
