package accrual

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/ryabkov82/gofermart/internal/app/models"
)

type AccrualClient struct {
	baseURL    string
	httpClient *http.Client
	limiter    *AdaptiveRateLimiter
}

func NewAccrualClient(baseURL string, initialRPS float64) *AccrualClient {
	return &AccrualClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		limiter: NewAdaptiveRateLimiter(initialRPS),
	}
}

func (c *AccrualClient) GetOrderInfo(ctx context.Context, orderNumber string) (*models.OrderAccrual, error) {

	// Ожидаем токен
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, err
	}

	// Реализация HTTP-запроса к внешнему API
	url := fmt.Sprintf("%s/api/orders/%s", c.baseURL, orderNumber)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Анализируем ответ и регулируем лимит
	c.limiter.AdjustBasedOnResponse(resp)

	switch resp.StatusCode {
	case http.StatusOK:
		var result models.OrderAccrual
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		return &result, nil

	case http.StatusNoContent:
		return nil, nil

	case http.StatusTooManyRequests:
		retryAfter := parseRetryAfterHeader(resp.Header.Get("Retry-After"))
		return nil, &RateLimitError{RetryAfter: retryAfter}

	case http.StatusInternalServerError:
		return nil, errors.New("accrual system internal error")

	default:
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

}

// parseRetryAfterHeader парсит заголовок Retry-After
func parseRetryAfterHeader(value string) time.Duration {
	if value == "" {
		return 60 * time.Second // default
	}

	if seconds, err := time.ParseDuration(value + "s"); err == nil {
		return seconds
	}

	if t, err := http.ParseTime(value); err == nil {
		return time.Until(t)
	}

	return 60 * time.Second // fallback to default
}

// RateLimitError ошибка при превышении лимита запросов
type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limit exceeded, retry after %v", e.RetryAfter)
}
