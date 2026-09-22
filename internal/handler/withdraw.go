package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"go.uber.org/zap"

	"github.com/yandex-praktikum/go-musthave-diploma-tpl/internal/middleware"
	"github.com/yandex-praktikum/go-musthave-diploma-tpl/internal/storage"
	"github.com/yandex-praktikum/go-musthave-diploma-tpl/internal/validator"
)

type WithdrawHandler struct {
	storage storage.WithdrawStorage
	logger  *zap.Logger
}

func NewWithdrawHandler(s storage.WithdrawStorage, l *zap.Logger) *WithdrawHandler {
	return &WithdrawHandler{storage: s, logger: l}
}

// ОБрабатывает POST /api/user/balance/withdraw
func (h *WithdrawHandler) WithdrawBalance(w http.ResponseWriter, r *http.Request) {
	login, ok := middleware.UserLoginFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Order string  `json:"order"`
		Sum   float64 `json:"sum"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Order == "" || req.Sum <= 0 || !validator.IsValidLuhn(req.Order) {
		http.Error(w, "Invalid order number or sum", http.StatusUnprocessableEntity)
		return
	}

	err := h.storage.WithdrawBalance(r.Context(), login, req.Order, req.Sum)
	if err != nil {
		if errors.Is(err, storage.ErrInsufficientFunds) {
			http.Error(w, "Insufficient funds", http.StatusPaymentRequired)
			return
		}

		if errors.Is(err, storage.ErrOrderAlreadyWithdrawn) {
			w.WriteHeader(http.StatusOK)
			return
		}

		h.logger.Error("Failed to withdraw balance", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Обрабатывает GET /api/user/withdrawals
func (h *WithdrawHandler) GetWithdrawals(w http.ResponseWriter, r *http.Request) {
	login, ok := middleware.UserLoginFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	withdrawals, err := h.storage.GetWithdrawalsByUser(r.Context(), login)
	if err != nil {
		h.logger.Error("Failed to get withdrawals", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if len(withdrawals) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(withdrawals); err != nil {
		h.logger.Error("Failed to encode withdrawals response", zap.Error(err))
	}
}
