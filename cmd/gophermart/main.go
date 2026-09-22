package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	"github.com/yandex-praktikum/go-musthave-diploma-tpl/internal/accrual"
	"github.com/yandex-praktikum/go-musthave-diploma-tpl/internal/config"
	"github.com/yandex-praktikum/go-musthave-diploma-tpl/internal/handler"
	"github.com/yandex-praktikum/go-musthave-diploma-tpl/internal/logger"
	"github.com/yandex-praktikum/go-musthave-diploma-tpl/internal/middleware"
	"github.com/yandex-praktikum/go-musthave-diploma-tpl/internal/storage"
	"github.com/yandex-praktikum/go-musthave-diploma-tpl/internal/worker"
)

func main() {
	cfg := config.New()

	//подключаем логгер
	logger, err := logger.New()
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}
	defer func() { _ = logger.Sync() }()

	logger.Info("Start Gophermart", zap.String("run_address", cfg.RunAddress))

	//подключаем БД
	dbStore, err := storage.NewPostgresStorage(cfg.DatabaseURI)
	if err != nil {
		logger.Fatal("Failed to connect to DB", zap.Error(err))
	}
	defer dbStore.Close()
	logger.Info("Success connect to DB")

	accrualClient := accrual.NewClient(cfg.AccrualAddress)
	accrualWorker := worker.NewAccrualWorker(dbStore, accrualClient, logger)

	//контекс для воркера
	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()

	go accrualWorker.Run(workerCtx)

	router := chi.NewRouter()

	router.Use(chimiddleware.Compress(5))
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.Info("incoming request", zap.String("method", r.Method), zap.String("path", r.URL.Path))
			next.ServeHTTP(w, r)
		})
	})

	authHandler := handler.NewAuthHandler(dbStore, logger)
	orderHandler := handler.NewOrderHandler(dbStore, logger)
	balanceHandler := handler.NewBalanceHandler(dbStore, logger)
	withdrawHandler := handler.NewWithdrawHandler(dbStore, logger)

	router.Post("/api/user/register", authHandler.Register)
	router.Post("/api/user/login", authHandler.Login)
	router.With(middleware.AuthMiddleware(logger)).Post("/api/user/orders", orderHandler.UploadOrder)
	router.With(middleware.AuthMiddleware(logger)).Get("/api/user/orders", orderHandler.GetOrders)
	router.With(middleware.AuthMiddleware(logger)).Get("/api/user/balance", balanceHandler.GetBalance)

	router.With(middleware.AuthMiddleware(logger)).Post("/api/user/balance/withdraw", withdrawHandler.WithdrawBalance)
	router.With(middleware.AuthMiddleware(logger)).Get("/api/user/withdrawals", withdrawHandler.GetWithdrawals)

	//healthcheck
	router.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})

	srv := &http.Server{
		Addr:         cfg.RunAddress,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("Server is start", zap.String("address", cfg.RunAddress))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server failed", zap.Error(err))
		}
	}()

	<-quit
	logger.Info("Shutting down server")
	workerCancel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) //5 секунд должно хватить
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Server shutdown", zap.Error(err))
	}
	logger.Info("Server exit")
}
