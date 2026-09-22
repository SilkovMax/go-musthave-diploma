package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"go.uber.org/zap"

	"github.com/yandex-praktikum/go-musthave-diploma-tpl/internal/middleware"
)

type BalanceStorage interface {
	GetUserBalance(ctx context.Context, login string) (float64, float64, error)
}

type BalanceHandler struct {
	storage BalanceStorage
	logger  *zap.Logger
}

func NewBalanceHandler(s BalanceStorage, l *zap.Logger) *BalanceHandler {
	return &BalanceHandler{storage: s, logger: l}
}

// Обрабатывает GET /api/user/balance
func (h *BalanceHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	login, ok := middleware.UserLoginFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	current, withdrawn, err := h.storage.GetUserBalance(r.Context(), login)
	if err != nil {
		h.logger.Error("Failed to get user balance", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := struct {
		Current   float64 `json:"current"`
		Withdrawn float64 `json:"withdrawn"`
	}{
		Current:   current,
		Withdrawn: withdrawn,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed balance response", zap.Error(err))
	}
}
