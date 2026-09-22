package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"go.uber.org/zap"

	"github.com/yandex-praktikum/go-musthave-diploma-tpl/internal/middleware"
	"github.com/yandex-praktikum/go-musthave-diploma-tpl/internal/storage"
	"github.com/yandex-praktikum/go-musthave-diploma-tpl/internal/validator"
)

type OrderStorage interface {
	AddOrder(ctx context.Context, login, number string) error
	GetOrdersByUser(ctx context.Context, login string) ([]storage.OrderResponse, error)
}

type OrderHandler struct {
	storage OrderStorage
	logger  *zap.Logger
}

func NewOrderHandler(s OrderStorage, l *zap.Logger) *OrderHandler {
	return &OrderHandler{storage: s, logger: l}
}

// Обрабатывает POST /api/user/orders
func (h *OrderHandler) UploadOrder(w http.ResponseWriter, r *http.Request) {
	login, ok := middleware.UserLoginFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Cannot read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	number := strings.TrimSpace(string(body))
	if number == "" {
		http.Error(w, "Order number is empty", http.StatusBadRequest)
		return
	}

	if !validator.IsValidLuhn(number) {
		http.Error(w, "Invalid order number", http.StatusUnprocessableEntity)
		return
	}

	err = h.storage.AddOrder(r.Context(), login, number)
	if err != nil {
		if errors.Is(err, storage.ErrOrderAlreadyProcessed) {
			w.WriteHeader(http.StatusOK)
			return
		}
		if errors.Is(err, storage.ErrOrderAlreadyUploadedByAnother) {
			http.Error(w, "Order already uploaded by another user", http.StatusConflict)
			return
		}
		h.logger.Error("Failed to add order", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// обрабатывает GET /api/user/orders
func (h *OrderHandler) GetOrders(w http.ResponseWriter, r *http.Request) {
	login, ok := middleware.UserLoginFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	orders, err := h.storage.GetOrdersByUser(r.Context(), login)
	if err != nil {
		h.logger.Error("Failed to get orders", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if len(orders) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(orders); err != nil {
		h.logger.Error("Failed to encode orders response", zap.Error(err))
	}
}
