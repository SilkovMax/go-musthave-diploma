package middleware

import (
	"context"
	"net/http"
	"strings"

	"go.uber.org/zap"
)

type ContextKey string

const UserLoginKey ContextKey = "userLogin"

func AuthMiddleware(logger *zap.Logger, storage interface {
	GetLoginByToken(ctx context.Context, token string) (string, error)
}) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var token string

			cookie, err := r.Cookie("auth_token")
			if err == nil && cookie.Value != "" {
				token = cookie.Value
			} else {
				authHeader := r.Header.Get("Authorization")
				if strings.HasPrefix(authHeader, "Bearer ") {
					token = strings.TrimPrefix(authHeader, "Bearer ")
				}
			}

			if token == "" {
				logger.Warn("Unauthorized: missing auth token", zap.String("path", r.URL.Path))
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			login, err := storage.GetLoginByToken(r.Context(), token)
			if err != nil {
				logger.Warn("Unauthorized: invalid token", zap.String("path", r.URL.Path))
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), UserLoginKey, login)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserLoginFromContext(ctx context.Context) (string, bool) {
	login, ok := ctx.Value(UserLoginKey).(string)
	return login, ok
}
