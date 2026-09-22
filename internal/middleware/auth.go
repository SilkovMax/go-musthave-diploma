package middleware

import (
	"context"
	"net/http"
	"strings"

	"go.uber.org/zap"
)

type contextKey string

const userLoginKey contextKey = "userLogin"

// проверяет наличие в куке авторазациорнных данных
func AuthMiddleware(logger *zap.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var login string

			cookie, err := r.Cookie("auth_token")
			if err == nil && cookie.Value != "" {
				login = cookie.Value
			} else {
				authHeader := r.Header.Get("Authorization")
				if strings.HasPrefix(authHeader, "Bearer ") {
					login = strings.TrimPrefix(authHeader, "Bearer ")
				}
			}

			// если логина нет, отклоняем запрос
			if login == "" {
				logger.Warn("Unauthorized error - no auth token", zap.String("path", r.URL.Path))
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userLoginKey, login)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserLoginFromContext(ctx context.Context) (string, bool) {
	login, ok := ctx.Value(userLoginKey).(string)
	return login, ok
}
