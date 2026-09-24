package handler

import (
	"context"
	"errors"
	"go.uber.org/zap"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yandex-praktikum/go-musthave-diploma-tpl/internal/middleware"
)

type mockBalanceStorage struct {
	current   float64
	withdrawn float64
	err       error
}

func (m *mockBalanceStorage) GetUserBalance(ctx context.Context, login string) (float64, float64, error) {
	return m.current, m.withdrawn, m.err
}

func TestBalanceHandler_GetBalance(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	tests := []struct {
		name           string
		current        float64
		withdrawn      float64
		expectedStatus int
	}{
		{
			name:           "Successful balance",
			current:        500.5,
			withdrawn:      100.0,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Zero balance",
			current:        0,
			withdrawn:      0,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &mockBalanceStorage{current: tt.current, withdrawn: tt.withdrawn}
			handler := NewBalanceHandler(mockStore, logger)

			req := httptest.NewRequest(http.MethodGet, "/api/user/balance", nil)
			ctx := context.WithValue(req.Context(), middleware.UserLoginKey, "testuser")
			req = req.WithContext(ctx)

			rr := httptest.NewRecorder()

			handler.GetBalance(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("handler wrong status code: got %v want %v", rr.Code, tt.expectedStatus)
			}
		})
	}
}

func TestBalanceHandler_GetBalance_Error(t *testing.T) {
	logger := zap.NewNop()

	mockStore := &mockBalanceStorage{
		err: errors.New("db connection failed"),
	}
	handler := NewBalanceHandler(mockStore, logger)

	req := httptest.NewRequest(http.MethodGet, "/api/user/balance", nil)
	ctx := context.WithValue(req.Context(), middleware.UserLoginKey, "testuser")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.GetBalance(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusInternalServerError)
	}
}
