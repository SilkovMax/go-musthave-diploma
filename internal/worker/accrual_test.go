package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/yandex-praktikum/go-musthave-diploma-tpl/internal/accrual"
	"github.com/yandex-praktikum/go-musthave-diploma-tpl/internal/storage"
)

// --- Моки ---

type mockWorkerStorage struct {
	orders        []storage.Order
	getErr        error
	updateErr     error
	updatedStatus string
}

func (m *mockWorkerStorage) GetOrdersForProcessing(ctx context.Context) ([]storage.Order, error) {
	return m.orders, m.getErr
}

func (m *mockWorkerStorage) UpdateOrderStatus(ctx context.Context, number, status string, accrual *float64) error {
	m.updatedStatus = status
	return m.updateErr
}

type mockWorkerClient struct {
	resp *accrual.OrderResponse
	err  error
}

func (m *mockWorkerClient) GetOrderStatus(ctx context.Context, orderNumber string) (*accrual.OrderResponse, error) {
	return m.resp, m.err
}

func TestNewAccrualWorker(t *testing.T) {
	logger := zap.NewNop()
	store := &mockWorkerStorage{}
	client := &mockWorkerClient{}

	w := NewAccrualWorker(store, client, logger)

	if w.pollInterval != 10*time.Second {
		t.Errorf("expected pollInterval 10s, got %v", w.pollInterval)
	}
	if w.storage == nil || w.client == nil || w.logger == nil {
		t.Error("dependencies should not be nil")
	}
}

// Тестируем когда заказов нет (len == 0)
func TestAccrualWorker_processOrders_Empty(t *testing.T) {
	logger := zap.NewNop()
	store := &mockWorkerStorage{orders: []storage.Order{}}
	client := &mockWorkerClient{}
	w := NewAccrualWorker(store, client, logger)

	w.processOrders(context.Background())
}

// Тестируем выход из цикла при отмене контекста
func TestAccrualWorker_processOrders_ContextCancel(t *testing.T) {
	logger := zap.NewNop()
	store := &mockWorkerStorage{
		orders: []storage.Order{{Number: "123", Status: "NEW"}},
	}
	client := &mockWorkerClient{
		resp: &accrual.OrderResponse{Order: "123", Status: "PROCESSED"},
	}
	w := NewAccrualWorker(store, client, logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	w.processOrders(ctx)
}

func TestAccrualWorker_processSingleOrder_RegisteredMapping(t *testing.T) {
	logger := zap.NewNop()
	store := &mockWorkerStorage{}
	client := &mockWorkerClient{
		resp: &accrual.OrderResponse{Order: "123", Status: "REGISTERED", Accrual: nil},
	}
	w := NewAccrualWorker(store, client, logger)

	w.processSingleOrder(context.Background(), storage.Order{Number: "123"})

	if store.updatedStatus != "PROCESSING" {
		t.Errorf("expected status to be mapped to PROCESSING, got %s", store.updatedStatus)
	}
}

// Тестируем успешное обновление статуса PROCESSED
func TestAccrualWorker_processSingleOrder_Success(t *testing.T) {
	logger := zap.NewNop()
	accrualVal := 500.0
	store := &mockWorkerStorage{}
	client := &mockWorkerClient{
		resp: &accrual.OrderResponse{Order: "123", Status: "PROCESSED", Accrual: &accrualVal},
	}
	w := NewAccrualWorker(store, client, logger)

	w.processSingleOrder(context.Background(), storage.Order{Number: "123"})

	if store.updatedStatus != "PROCESSED" {
		t.Errorf("expected status PROCESSED, got %s", store.updatedStatus)
	}
}

// Тестируем  204 (заказ не найден)
func TestAccrualWorker_processSingleOrder_NotFound(t *testing.T) {
	logger := zap.NewNop()
	store := &mockWorkerStorage{}
	client := &mockWorkerClient{
		err: accrual.ErrOrderNotFound,
	}
	w := NewAccrualWorker(store, client, logger)

	w.processSingleOrder(context.Background(), storage.Order{Number: "123"})

	if store.updatedStatus != "" {
		t.Error("UpdateOrderStatus should NOT have been called for ErrOrderNotFound")
	}
}

// Тестируем 500
func TestAccrualWorker_processSingleOrder_ServerError(t *testing.T) {
	logger := zap.NewNop()
	store := &mockWorkerStorage{}
	client := &mockWorkerClient{
		err: errors.New("internal server error"),
	}
	w := NewAccrualWorker(store, client, logger)

	w.processSingleOrder(context.Background(), storage.Order{Number: "123"})

	if store.updatedStatus != "" {
		t.Error("UpdateOrderStatus should NOT have been called for server error")
	}
}

func TestAccrualWorker_Run_ImmediateExit(t *testing.T) {
	logger := zap.NewNop()
	store := &mockWorkerStorage{}
	client := &mockWorkerClient{}
	w := NewAccrualWorker(store, client, logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	w.Run(ctx)
}
