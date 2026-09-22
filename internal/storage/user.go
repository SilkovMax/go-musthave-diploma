package storage

import (
	"context"
)

type User struct {
	Login    string
	Password string
}

type UserStorage interface {
	CreateUser(ctx context.Context, login, passwordHash string) error
	GetUser(ctx context.Context, login string) (User, error)
}
