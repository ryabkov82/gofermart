package accrual

import (
	"context"
	"net/http"
	"sync"
	"time"
)

type AdaptiveRateLimiter struct {
	mu                sync.Mutex
	requestsPerSecond float64       // Текущий лимит
	minRPS            float64       // Минимальный лимит (0.1 запр/сек)
	maxRPS            float64       // Максимальный лимит (100 запр/сек)
	lastRateLimit     time.Time     // Время последнего 429
	cooldownPeriod    time.Duration // Период ожидания после 429 (60 сек)
	tokenBucket       chan struct{} // Токен-бакет
	closeChan         chan struct{}
}

func NewAdaptiveRateLimiter(initialRPS float64) *AdaptiveRateLimiter {
	limiter := &AdaptiveRateLimiter{
		requestsPerSecond: initialRPS,
		minRPS:            0.1,
		maxRPS:            100,
		cooldownPeriod:    60 * time.Second,
		tokenBucket:       make(chan struct{}, int(initialRPS)),
		closeChan:         make(chan struct{}),
	}

	// Наполняем bucket
	for i := 0; i < int(initialRPS); i++ {
		limiter.tokenBucket <- struct{}{}
	}

	// Запускаем наполнение bucket
	go limiter.refillBucket()

	return limiter
}

func (l *AdaptiveRateLimiter) refillBucket() {
	ticker := time.NewTicker(time.Second / time.Duration(l.requestsPerSecond))
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			select {
			case l.tokenBucket <- struct{}{}:
			default: // bucket полон
			}
		case <-l.closeChan:
			return
		}
	}
}

func (l *AdaptiveRateLimiter) Wait(ctx context.Context) error {
	select {
	case <-l.tokenBucket:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (l *AdaptiveRateLimiter) AdjustBasedOnResponse(resp *http.Response) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if resp.StatusCode == http.StatusTooManyRequests {
		now := time.Now()
		if now.Sub(l.lastRateLimit) < l.cooldownPeriod {
			// Уменьшаем лимит вдвое, но не ниже minRPS
			l.requestsPerSecond = max(l.requestsPerSecond/2, l.minRPS)
		}
		l.lastRateLimit = now

		// Обновляем интервал наполнения
		l.resizeBucket(int(l.requestsPerSecond))

		// Получаем Retry-After из заголовка
		if retryAfter := parseRetryAfter(resp.Header.Get("Retry-After")); retryAfter > 0 {
			time.Sleep(retryAfter)
		}
	} else {
		// Постепенно увеличиваем лимит при успешных запросах
		if time.Since(l.lastRateLimit) > l.cooldownPeriod {
			l.requestsPerSecond = min(l.requestsPerSecond*1.1, l.maxRPS)
			l.resizeBucket(int(l.requestsPerSecond))
		}
	}
}

func (l *AdaptiveRateLimiter) resizeBucket(newSize int) {
	// Увеличиваем или уменьшаем размер bucket
	for len(l.tokenBucket) > newSize {
		<-l.tokenBucket
	}

	// Создаем новый канал нужного размера
	newBucket := make(chan struct{}, newSize)
	for i := 0; i < newSize; i++ {
		newBucket <- struct{}{}
	}

	l.tokenBucket = newBucket
}

func (l *AdaptiveRateLimiter) Close() {
	close(l.closeChan)
}

// parseRetryAfter парсит заголовок Retry-After
func parseRetryAfter(value string) time.Duration {
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
