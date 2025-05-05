package getorders

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/ryabkov82/gofermart/internal/app/models"
)

type OrderResponse struct {
	Number     string             `json:"number"`
	Status     models.OrderStatus `json:"status"`
	Accrual    float64            `json:"accrual,omitempty"`
	UploadedAt string             `json:"uploaded_at"`
}

type URLHandler interface {
	GetUserOrders(context.Context) ([]models.Order, error)
}

func GetHandler(urlHandler URLHandler, log *zap.Logger) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {

		orders, err := urlHandler.GetUserOrders(req.Context())
		if err != nil {
			res.WriteHeader(http.StatusInternalServerError)
			log.Error("Failed to get user orders", zap.Error(err))
			return
		}

		if len(orders) == 0 {
			res.WriteHeader(http.StatusNoContent)
			return
		}

		response := make([]OrderResponse, len(orders))
		for i, order := range orders {
			response[i] = OrderResponse{
				Number:     order.Number,
				Status:     order.Status,
				Accrual:    order.Accrual,
				UploadedAt: order.UploadedAt.Format(time.RFC3339),
			}
		}

		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusOK)
		json.NewEncoder(res).Encode(response)

	}
}
