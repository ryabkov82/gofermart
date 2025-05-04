package getbalance

import (
	"context"
	"encoding/json"
	"net/http"

	"go.uber.org/zap"

	"github.com/ryabkov82/gofermart/internal/app/models"
)

type URLHandler interface {
	GetUserBalance(context.Context) (models.Balance, error)
}

func GetHandler(urlHandler URLHandler, log *zap.Logger) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {

		balance, err := urlHandler.GetUserBalance(req.Context())
		if err != nil {
			res.WriteHeader(http.StatusInternalServerError)
			log.Error("Failed to get user balance", zap.Error(err))
			return
		}

		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusOK)
		json.NewEncoder(res).Encode(balance)

	}
}
