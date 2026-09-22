package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"

	"go.uber.org/zap"

	"github.com/yandex-praktikum/go-musthave-diploma-tpl/internal/storage"
)

type AuthHandler struct {
	storage storage.UserStorage
	logger  *zap.Logger
}

func NewAuthHandler(s storage.UserStorage, l *zap.Logger) *AuthHandler {
	return &AuthHandler{storage: s, logger: l}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Login == "" || req.Password == "" {
		http.Error(w, "Login and password cannot be empty", http.StatusBadRequest)
		return
	}

	hash := sha256.Sum256([]byte(req.Password))
	hashedPassword := hex.EncodeToString(hash[:])

	// Сохраняем в хранилище
	err := h.storage.CreateUser(r.Context(), req.Login, hashedPassword)
	if err != nil {
		if errors.Is(err, storage.ErrUserAlreadyExists) {
			http.Error(w, "User already exists", http.StatusConflict)
			return
		}
		h.logger.Error("Failed to create user", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    req.Login,
		Path:     "/",
		HttpOnly: true,
	})

	w.WriteHeader(http.StatusOK)
}

// Обрабатывает POST /api/user/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Login == "" || req.Password == "" {
		http.Error(w, "Login and password cannot be empty", http.StatusBadRequest)
		return
	}

	user, err := h.storage.GetUser(r.Context(), req.Login)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {

			http.Error(w, "Invalid login or password", http.StatusUnauthorized)
			return
		}
		h.logger.Error("Failed to get user", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	hash := sha256.Sum256([]byte(req.Password))
	hashedPassword := hex.EncodeToString(hash[:])

	if user.Password != hashedPassword {
		http.Error(w, "Invalid login or password", http.StatusUnauthorized)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    req.Login,
		Path:     "/",
		HttpOnly: true,
	})

	w.WriteHeader(http.StatusOK)
}
