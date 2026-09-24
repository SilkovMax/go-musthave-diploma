package handler

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yandex-praktikum/go-musthave-diploma-tpl/internal/middleware"
	"go.uber.org/zap"

	"github.com/yandex-praktikum/go-musthave-diploma-tpl/internal/storage"
)

type mockOrderStorage struct {
	addOrderErr error
}

func (m *mockOrderStorage) AddOrder(ctx context.Context, login, number string) error {
	return m.addOrderErr
}

func (m *mockOrderStorage) GetOrdersByUser(ctx context.Context, login string) ([]storage.OrderResponse, error) {
	return nil, nil
}

func TestOrderHandler_UploadOrder(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	tests := []struct {
		name           string
		body           string
		mockErr        error
		expectedStatus int
	}{
		{
			name:           "New order accepted",
			body:           "9278923470",
			mockErr:        nil,
			expectedStatus: http.StatusAccepted,
		},
		{
			name:           "Order already processed by this user",
			body:           "9278923470",
			mockErr:        storage.ErrOrderAlreadyProcessed,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Order uploaded by another user",
			body:           "9278923470",
			mockErr:        storage.ErrOrderAlreadyUploadedByAnother,
			expectedStatus: http.StatusConflict,
		},
		{
			name:           "Invalid Luhn number",
			body:           "123456789", // невалидный номер
			mockErr:        nil,
			expectedStatus: http.StatusUnprocessableEntity,
		},
		{
			name:           "Empty body",
			body:           "",
			mockErr:        nil,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &mockOrderStorage{addOrderErr: tt.mockErr}
			handler := NewOrderHandler(mockStore, logger)

			req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "text/plain")

			ctx := context.WithValue(req.Context(), middleware.UserLoginKey, "testuser")
			req = req.WithContext(ctx)

			rr := httptest.NewRecorder()

			handler.UploadOrder(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, tt.expectedStatus)
			}
		})
	}

}

func TestOrderHandler_GetOrders_Error(t *testing.T) {
	logger := zap.NewNop()

	// Создаем специальный мок, который возвращает ошибку
	mockStore := &mockOrderStorageError{}
	handler := NewOrderHandler(mockStore, logger)

	req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
	ctx := context.WithValue(req.Context(), middleware.UserLoginKey, "testuser")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.GetOrders(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusInternalServerError)
	}
}

// Вспомогательный мок для ошибки
type mockOrderStorageError struct{}

func (m *mockOrderStorageError) AddOrder(ctx context.Context, login, number string) error { return nil }
func (m *mockOrderStorageError) GetOrdersByUser(ctx context.Context, login string) ([]storage.OrderResponse, error) {
	return nil, errors.New("db error")
}
