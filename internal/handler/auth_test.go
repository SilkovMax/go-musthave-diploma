package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"github.com/yandex-praktikum/go-musthave-diploma-tpl/internal/storage"
)

type mockUserStorage struct {
	createUserErr error
	getUserErr    error
	getUserResult storage.User
	createSessErr error
}

func (m *mockUserStorage) CreateUser(ctx context.Context, login, passwordHash string) error {
	return m.createUserErr
}

func (m *mockUserStorage) GetUser(ctx context.Context, login string) (storage.User, error) {
	if m.getUserErr != nil {
		return storage.User{}, m.getUserErr
	}
	return m.getUserResult, nil
}

func (m *mockUserStorage) CreateSession(ctx context.Context, login, token string) error {
	return m.createSessErr
}

func TestAuthHandler_Register(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	tests := []struct {
		name           string
		body           string
		mockErr        error
		expectedStatus int
	}{
		{"Successful registration", `{"login":"test","password":"pass"}`, nil, http.StatusOK},
		{"User already exists", `{"login":"test","password":"pass"}`, storage.ErrUserAlreadyExists, http.StatusConflict},
		{"Invalid JSON", `{"login":"test"`, nil, http.StatusBadRequest},
		{"Empty login", `{"login":"","password":"pass"}`, nil, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &mockUserStorage{createUserErr: tt.mockErr}
			handler := NewAuthHandler(mockStore, logger)

			req := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			handler.Register(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("got %v want %v", rr.Code, tt.expectedStatus)
			}
		})
	}
}

func TestAuthHandler_Login(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	validHash := "5e884898da28047151d0e56f8dc6292773603d0d6aabbdd62a11ef721d1542d8"

	tests := []struct {
		name           string
		body           string
		mockErr        error
		mockUser       storage.User
		expectedStatus int
	}{
		{
			name:           "Successful login",
			body:           `{"login":"test","password":"password"}`,
			mockErr:        nil,
			mockUser:       storage.User{Login: "test", Password: validHash},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "User not found",
			body:           `{"login":"test","password":"password"}`,
			mockErr:        storage.ErrUserNotFound,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Wrong password",
			body:           `{"login":"test","password":"wrongpass"}`,
			mockErr:        nil,
			mockUser:       storage.User{Login: "test", Password: validHash},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Invalid JSON",
			body:           `{"login":"test"`,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &mockUserStorage{getUserErr: tt.mockErr, getUserResult: tt.mockUser}
			handler := NewAuthHandler(mockStore, logger)

			req := httptest.NewRequest(http.MethodPost, "/api/user/login", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			handler.Login(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("got %v want %v", rr.Code, tt.expectedStatus)
			}
		})
	}
}
