package storage

import (
	"context"
	"time"
)

type WithdrawalResponse struct {
	Order       string    `json:"order"`
	Sum         float64   `json:"sum"`
	ProcessedAt time.Time `json:"processed_at"`
}

type WithdrawStorage interface {
	WithdrawBalance(ctx context.Context, login, orderNumber string, sum float64) error
	GetWithdrawalsByUser(ctx context.Context, login string) ([]WithdrawalResponse, error)
}
