package withdrawals

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/ryabkov82/gofermart/internal/app/models"
	"go.uber.org/zap"
)

type WithdrawalResponse struct {
	Order       string  `json:"order"`
	Sum         float64 `json:"sum"`
	ProcessedAt string  `json:"processed_at"`
}

type URLHandler interface {
	GetWithdrawals(ctx context.Context) ([]models.Withdrawal, error)
}

func GetHandler(urlHandler URLHandler, log *zap.Logger) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {

		withdrawals, err := urlHandler.GetWithdrawals(req.Context())
		if err != nil {
			res.WriteHeader(http.StatusInternalServerError)
			log.Error("Failed to get user withdrawals", zap.Error(err))
			return
		}

		if len(withdrawals) == 0 {
			res.WriteHeader(http.StatusNoContent)
			return
		}

		response := make([]WithdrawalResponse, len(withdrawals))
		for i, withdrawal := range withdrawals {
			response[i] = WithdrawalResponse{
				Order:       withdrawal.Order,
				Sum:         withdrawal.Sum,
				ProcessedAt: withdrawal.ProcessedAt.Format(time.RFC3339),
			}
		}

		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusOK)
		json.NewEncoder(res).Encode(response)

	}
}
