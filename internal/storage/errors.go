package storage

import "errors"

// Ошибки пользователей
var (
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrUserNotFound      = errors.New("user not found")
)

// Ошибки заказов
var (
	ErrOrderAlreadyProcessed         = errors.New("order already processed")
	ErrOrderAlreadyUploadedByAnother = errors.New("order already uploaded by another user")
)

// Ошибки списаний баланса
var (
	ErrInsufficientFunds     = errors.New("not enough points")
	ErrOrderAlreadyWithdrawn = errors.New("order already used")
)
