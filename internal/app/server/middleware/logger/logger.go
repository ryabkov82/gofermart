package logger

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

func RequestLogging(log *zap.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		fn := func(w http.ResponseWriter, r *http.Request) {

			// создаем обертку вокруг `http.ResponseWriter`
			// для получения сведений об ответе
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			// Момент получения запроса, чтобы вычислить время обработки
			t1 := time.Now()

			// Запись отправится в лог в defer
			// в этот момент запрос уже будет обработан
			defer func() {
				log.Info("request completed",
					zap.String("method", r.Method),
					zap.String("path", r.URL.Path),
					zap.Int("status", ww.Status()),
					zap.Int("bytes", ww.BytesWritten()),
					zap.String("duration", time.Since(t1).String()),
				)
			}()

			// Передаем управление следующему обработчику в цепочке middleware
			next.ServeHTTP(ww, r)
		}

		// Возвращаем созданный выше обработчик, приведя его к типу http.HandlerFunc
		return http.HandlerFunc(fn)
	}
}
