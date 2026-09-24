package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"github.com/yandex-praktikum/go-musthave-diploma-tpl/internal/middleware"
	"github.com/yandex-praktikum/go-musthave-diploma-tpl/internal/storage"
)

type mockWithdrawStorage struct {
	withdrawErr error
}

func (m *mockWithdrawStorage) WithdrawBalance(ctx context.Context, login, orderNumber string, sum float64) error {
	return m.withdrawErr
}

func (m *mockWithdrawStorage) GetWithdrawalsByUser(ctx context.Context, login string) ([]storage.WithdrawalResponse, error) {
	return nil, nil
}

func TestWithdrawHandler_WithdrawBalance(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	tests := []struct {
		name           string
		body           string
		mockErr        error
		expectedStatus int
	}{
		{
			name:           "Successful withdrawal",
			body:           `{"order":"9278923470","sum":100}`,
			mockErr:        nil,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Insufficient funds",
			body:           `{"order":"9278923470","sum":10000}`,
			mockErr:        storage.ErrInsufficientFunds,
			expectedStatus: http.StatusPaymentRequired,
		},
		{
			name:           "Order already withdrawn",
			body:           `{"order":"9278923470","sum":100}`,
			mockErr:        storage.ErrOrderAlreadyWithdrawn,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Invalid JSON",
			body:           `{"order":"9278923470"`,
			mockErr:        nil,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Invalid Luhn number",
			body:           `{"order":"12345","sum":100}`,
			mockErr:        nil,
			expectedStatus: http.StatusUnprocessableEntity,
		},
		{
			name:           "Negative sum",
			body:           `{"order":"9278923470","sum":-50}`,
			mockErr:        nil,
			expectedStatus: http.StatusUnprocessableEntity,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &mockWithdrawStorage{withdrawErr: tt.mockErr}
			handler := NewWithdrawHandler(mockStore, logger)

			req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")

			ctx := context.WithValue(req.Context(), middleware.UserLoginKey, "testuser")
			req = req.WithContext(ctx)

			rr := httptest.NewRecorder()

			handler.WithdrawBalance(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, tt.expectedStatus)
			}
		})
	}
}
