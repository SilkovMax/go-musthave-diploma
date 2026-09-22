package storage

import (
	"context"
	"sync"
)

type OrderStorage interface {
	AddOrder(ctx context.Context, login, number string) error
}

type MemoryOrderStorage struct {
	mu     sync.RWMutex
	orders map[string]string
}

func NewMemoryOrderStorage() *MemoryOrderStorage {
	return &MemoryOrderStorage{
		orders: make(map[string]string),
	}
}

func (m *MemoryOrderStorage) AddOrder(ctx context.Context, login, number string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existingLogin, exists := m.orders[number]
	if exists {
		if existingLogin == login {
			return ErrOrderAlreadyProcessed
		}
		return ErrOrderAlreadyUploadedByAnother
	}

	m.orders[number] = login
	return nil
}
