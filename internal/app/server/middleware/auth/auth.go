package auth

import (
	"context"
	"net/http"

	"github.com/ryabkov82/gofermart/internal/app/utils/jwtauth"
)

func AuthMiddleware(jwtKey []byte) func(next http.Handler) http.Handler {
	// Middleware: проверяет JWT
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {

			cookie, err := r.Cookie("token")
			if err != nil || cookie == nil {
				http.Error(w, "Authorization token required", http.StatusUnauthorized)
				return
			}

			tokenStr := cookie.Value

			userID, err := jwtauth.ValidateTokenAndGetUserID(tokenStr, jwtKey)

			if err != nil {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), jwtauth.UserIDContextKey, userID)
			r = r.WithContext(ctx)
			// Передаем управление следующему обработчику в цепочке middleware
			next.ServeHTTP(w, r)
		}

		// Возвращаем созданный выше обработчик, приведя его к типу http.HandlerFunc
		return http.HandlerFunc(fn)

	}
}
