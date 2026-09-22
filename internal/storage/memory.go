package storage

import (
	"context"
	"sync"
)

type MemoryStorage struct {
	mu    sync.RWMutex
	users map[string]User
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		users: make(map[string]User),
	}
}

func (m *MemoryStorage) CreateUser(ctx context.Context, login, passwordHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.users[login]; exists {
		return ErrUserAlreadyExists
	}

	m.users[login] = User{
		Login:    login,
		Password: passwordHash,
	}
	return nil
}

func (m *MemoryStorage) GetUser(ctx context.Context, login string) (User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	u, exists := m.users[login]
	if !exists {
		return User{}, ErrUserNotFound
	}
	return u, nil
}
