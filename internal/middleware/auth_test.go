package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

type mockAuthStorage struct {
	login string
	err   error
}

func (m *mockAuthStorage) GetLoginByToken(ctx context.Context, token string) (string, error) {
	return m.login, m.err
}

func TestAuthMiddleware(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	t.Run("No token provided", func(t *testing.T) {
		store := &mockAuthStorage{}
		mw := AuthMiddleware(logger, store)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rr := httptest.NewRecorder()

		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("Valid token in cookie", func(t *testing.T) {
		store := &mockAuthStorage{login: "testuser", err: nil}
		mw := AuthMiddleware(logger, store)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: "valid_token"})
		rr := httptest.NewRecorder()

		nextCalled := false
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			login, ok := UserLoginFromContext(r.Context())
			if !ok || login != "testuser" {
				t.Errorf("expected login 'testuser', got '%s', ok: %v", login, ok)
			}
		})).ServeHTTP(rr, req)

		if !nextCalled {
			t.Error("next handler was not called")
		}
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("Invalid token (storage error)", func(t *testing.T) {
		store := &mockAuthStorage{err: errors.New("not found")}
		mw := AuthMiddleware(logger, store)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: "invalid_token"})
		rr := httptest.NewRecorder()

		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
	})
}
