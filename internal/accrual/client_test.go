package accrual

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient_GetOrderStatus(t *testing.T) {
	// тестовый HTTP-сервер, который будет имитировать внешний сервис
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/orders/valid_order" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"order": "valid_order", "status": "PROCESSED", "accrual": 500}`))
		} else if r.URL.Path == "/api/orders/not_found" {
			w.WriteHeader(http.StatusNoContent)
		} else if r.URL.Path == "/api/orders/rate_limited" {
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	t.Run("Success 200", func(t *testing.T) {
		resp, err := client.GetOrderStatus(ctx, "valid_order")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Status != "PROCESSED" || *resp.Accrual != 500 {
			t.Errorf("unexpected response: %+v", resp)
		}
	})

	t.Run("Not Found 204", func(t *testing.T) {
		_, err := client.GetOrderStatus(ctx, "not_found")
		if err == nil || err != ErrOrderNotFound {
			t.Errorf("expected ErrOrderNotFound, got %v", err)
		}
	})

	t.Run("Rate Limited 429", func(t *testing.T) {
		_, err := client.GetOrderStatus(ctx, "rate_limited")
		if err == nil || !errors.Is(err, ErrTooManyRequests) {
			t.Errorf("expected ErrTooManyRequests, got %v", err)
		}
	})

	t.Run("Server Error 500", func(t *testing.T) {
		_, err := client.GetOrderStatus(ctx, "error")
		if err == nil {
			t.Errorf("expected error for 500 status")
		}
	})
}

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
	}{
		{"Valid", "60", 60 * time.Second},
		{"Empty", "", 60 * time.Second},
		{"Invalid", "abc", 60 * time.Second},
		{"Negative", "-10", 60 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if result := ParseRetryAfter(tt.input); result != tt.expected {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}
