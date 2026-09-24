package worker

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"

	"github.com/yandex-praktikum/go-musthave-diploma-tpl/internal/accrual"
	"github.com/yandex-praktikum/go-musthave-diploma-tpl/internal/storage"
)

type AccrualClient interface {
	GetOrderStatus(ctx context.Context, orderNumber string) (*accrual.OrderResponse, error)
}

// фоновый воркер для проверки статусов заказов
type AccrualWorker struct {
	storage      storage.OrderProcessor
	client       AccrualClient
	logger       *zap.Logger
	pollInterval time.Duration
}

func NewAccrualWorker(s storage.OrderProcessor, client AccrualClient, logger *zap.Logger) *AccrualWorker {
	return &AccrualWorker{
		storage:      s,
		client:       client,
		logger:       logger,
		pollInterval: 10 * time.Second, // Захардкожено 10 секунд
	}
}

func (w *AccrualWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	w.logger.Info("Accrual worker start", zap.Duration("poll_interval", w.pollInterval))

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("Accrual worker stop")
			return
		case <-ticker.C:
			w.processOrders(ctx)
		}
	}
}

func (w *AccrualWorker) processOrders(ctx context.Context) {
	orders, err := w.storage.GetOrdersForProcessing(ctx)
	if err != nil {
		w.logger.Error("Failed to get orders", zap.Error(err))
		return
	}

	if len(orders) == 0 {
		return
	}

	w.logger.Debug("Work orders", zap.Int("count", len(orders)))

	for _, order := range orders {
		select {
		case <-ctx.Done():
			return
		default:
			w.processSingleOrder(ctx, order)
			time.Sleep(1 * time.Second)
		}
	}
}

// обрабатывает один заказ
func (w *AccrualWorker) processSingleOrder(ctx context.Context, order storage.Order) {
	resp, err := w.client.GetOrderStatus(ctx, order.Number)
	if err != nil {
		if errors.Is(err, accrual.ErrTooManyRequests) {
			w.logger.Warn("RL, stop worker 60s")
			time.Sleep(60 * time.Second)
			return
		}
		if errors.Is(err, accrual.ErrOrderNotFound) {
			w.logger.Debug("Order not found", zap.String("order", order.Number))
			return
		}
		w.logger.Error("Failed to get order status", zap.String("order", order.Number), zap.Error(err))
		return
	}

	if resp.Status == "REGISTERED" || resp.Status == "PROCESSING" {
		resp.Status = "PROCESSING"
	}

	err = w.storage.UpdateOrderStatus(ctx, order.Number, resp.Status, resp.Accrual)

	if err != nil {
		w.logger.Error("Failed to update order status", zap.String("order", order.Number), zap.Error(err))
		return
	}

	w.logger.Info("Order status updated",
		zap.String("order", order.Number),
		zap.String("status", resp.Status),
	)
}
