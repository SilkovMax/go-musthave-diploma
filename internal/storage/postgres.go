package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type OrderResponse struct {
	Number     string    `json:"number"`
	Status     string    `json:"status"`
	Accrual    float64   `json:"accrual"`
	UploadedAt time.Time `json:"uploaded_at"`
}

type Order struct {
	Number      string
	UserLogin   string
	Status      string
	BonusAmount float64
}

type OrderProcessor interface {
	GetOrdersForProcessing(ctx context.Context) ([]Order, error)
	UpdateOrderStatus(ctx context.Context, number, status string, accrual *float64) error
}

type PostgresStorage struct {
	db *sql.DB
}

func NewPostgresStorage(dsn string) (*PostgresStorage, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("ошибка инициализации пула соединений: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("не удалось подключиться к БД: %w", err)
	}

	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("ошибка применения миграций: %w", err)
	}

	return &PostgresStorage{db: db}, nil
}

func (s *PostgresStorage) Close() error {
	return s.db.Close()
}

func runMigrations(db *sql.DB) error {
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("ошибка создания драйвера миграций: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://./migrations",
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("ошибка создания экземпляра мигратора: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("ошибка применения миграций: %w", err)
	}

	return nil
}

// как в прошлом проекте с retry
func withRetry(operationName string, fn func() error) error {
	delays := []time.Duration{time.Second, 3 * time.Second, 5 * time.Second}

	for attempt := 0; attempt < 4; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		if !isRetriable(err) {
			log.Printf("DBStorage.%s: критическая ошибка: %v", operationName, err)
			return err
		}

		if attempt < 3 {
			log.Printf("DBStorage.%s: retry %d/3 (ошибка: %v)", operationName, attempt+1, err)
			time.Sleep(delays[attempt])
		} else {
			log.Printf("DBStorage.%s: все retry провалились: %v", operationName, err)
		}
	}
	return fmt.Errorf("все retry провалились для %s", operationName)
}

func isRetriable(err error) bool {
	if err == nil {
		return false
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if strings.HasPrefix(pgErr.Code, "08") {
			return true
		}
	}

	var netErr interface{ Timeout() bool }
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	return false
}

func (s *PostgresStorage) CreateUser(ctx context.Context, login, passwordHash string) error {
	query := `INSERT INTO users (login, password_hash) VALUES ($1, $2)`

	err := withRetry("CreateUser", func() error {
		_, err := s.db.ExecContext(ctx, query, login, passwordHash)
		return err
	})

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return ErrUserAlreadyExists
		}
		return err
	}
	return nil
}

func (s *PostgresStorage) GetUser(ctx context.Context, login string) (User, error) {
	var user User
	query := `SELECT login, password_hash FROM users WHERE login = $1`

	err := withRetry("GetUser", func() error {
		return s.db.QueryRowContext(ctx, query, login).Scan(&user.Login, &user.Password)
	})

	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	return user, err
}

func (s *PostgresStorage) AddOrder(ctx context.Context, login, number string) error {
	var existingLogin string

	err := s.db.QueryRowContext(ctx, "SELECT user_login FROM orders WHERE number = $1", number).Scan(&existingLogin)

	if err == nil {
		if existingLogin == login {
			return ErrOrderAlreadyProcessed
		}
		return ErrOrderAlreadyUploadedByAnother
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	query := `INSERT INTO orders (number, user_login, status, bonus_amount) VALUES ($1, $2, 'NEW', 0)`
	err = withRetry("AddOrder", func() error {
		_, err := s.db.ExecContext(ctx, query, number, login)
		return err
	})

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return ErrOrderAlreadyUploadedByAnother
		}
		return err
	}

	return nil
}

func (s *PostgresStorage) GetOrdersByUser(ctx context.Context, login string) ([]OrderResponse, error) {
	query := `SELECT number, status, bonus_amount, uploaded_at
	          FROM orders
	          WHERE user_login = $1
	          ORDER BY uploaded_at DESC`

	var orders []OrderResponse
	err := withRetry("GetOrdersByUser", func() error {
		rows, err := s.db.QueryContext(ctx, query, login)
		if err != nil {
			return err
		}
		defer rows.Close()

		orders = nil
		for rows.Next() {
			var o OrderResponse
			if err := rows.Scan(&o.Number, &o.Status, &o.Accrual, &o.UploadedAt); err != nil {
				return err
			}
			orders = append(orders, o)
		}
		return rows.Err()
	})

	return orders, err
}

func (s *PostgresStorage) GetUserBalance(ctx context.Context, login string) (float64, float64, error) {
	var accrued, withdrawn float64

	err := withRetry("GetUserBalance", func() error {
		err := s.db.QueryRowContext(ctx, `
			SELECT COALESCE(SUM(bonus_amount), 0)
			FROM orders
			WHERE user_login = $1 AND status = 'PROCESSED'
		`, login).Scan(&accrued)
		if err != nil {
			return err
		}

		return s.db.QueryRowContext(ctx, `
			SELECT COALESCE(SUM(sum), 0)
			FROM withdrawals
			WHERE user_login = $1
		`, login).Scan(&withdrawn)
	})

	current := accrued - withdrawn

	return current, withdrawn, err
}

func (s *PostgresStorage) GetOrdersForProcessing(ctx context.Context) ([]Order, error) {
	query := `SELECT number, user_login, status, bonus_amount FROM orders WHERE status IN ('NEW', 'PROCESSING') LIMIT 10` //10 чтобы не перегрузить бд

	var orders []Order
	err := withRetry("GetOrdersForProcessing", func() error {
		rows, err := s.db.QueryContext(ctx, query)
		if err != nil {
			return err
		}
		defer rows.Close()

		orders = nil
		for rows.Next() {
			var o Order
			if err := rows.Scan(&o.Number, &o.UserLogin, &o.Status, &o.BonusAmount); err != nil {
				return err
			}
			orders = append(orders, o)
		}
		return rows.Err()
	})

	return orders, err
}

func (s *PostgresStorage) UpdateOrderStatus(ctx context.Context, number, status string, accrual *float64) error {
	query := `UPDATE orders SET status = $1, bonus_amount = COALESCE($2, bonus_amount) WHERE number = $3`

	err := withRetry("UpdateOrderStatus", func() error {
		_, err := s.db.ExecContext(ctx, query, status, accrual, number)
		return err
	})

	return err
}

// списание баллов с баланса пользователя
func (s *PostgresStorage) WithdrawBalance(ctx context.Context, login, orderNumber string, sum float64) error {
	return withRetry("WithdrawBalance", func() error {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("начало транзакции: %w", err)
		}
		defer tx.Rollback()

		_, err = tx.ExecContext(ctx, `SELECT 1 FROM users WHERE login = $1 FOR UPDATE`, login)
		if err != nil {
			return fmt.Errorf("блокировка пользователя: %w", err)
		}

		var currentBalance float64
		err = tx.QueryRowContext(ctx, `
			SELECT COALESCE(SUM(bonus_amount), 0)
			FROM orders
			WHERE user_login = $1 AND status = 'PROCESSED'
		`, login).Scan(&currentBalance)
		if err != nil {
			return fmt.Errorf("получение баланса: %w", err)
		}

		var withdrawnSum float64
		err = tx.QueryRowContext(ctx, `
			SELECT COALESCE(SUM(sum), 0)
			FROM withdrawals
			WHERE user_login = $1
		`, login).Scan(&withdrawnSum)
		if err != nil {
			return fmt.Errorf("получение суммы списаний: %w", err)
		}

		availableBalance := currentBalance - withdrawnSum
		if availableBalance < sum {
			return ErrInsufficientFunds
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO withdrawals (user_login, order_number, sum)
			VALUES ($1, $2, $3)
		`, login, orderNumber, sum)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
				return ErrOrderAlreadyWithdrawn
			}
			return fmt.Errorf("вставка списания: %w", err)
		}

		return tx.Commit()
	})
}

func (s *PostgresStorage) GetWithdrawalsByUser(ctx context.Context, login string) ([]WithdrawalResponse, error) {
	query := `SELECT order_number, sum, processed_at
	          FROM withdrawals
	          WHERE user_login = $1
	          ORDER BY processed_at DESC`

	var withdrawals []WithdrawalResponse
	err := withRetry("GetWithdrawalsByUser", func() error {
		rows, err := s.db.QueryContext(ctx, query, login)
		if err != nil {
			return err
		}
		defer rows.Close()

		withdrawals = nil
		for rows.Next() {
			var w WithdrawalResponse
			if err := rows.Scan(&w.Order, &w.Sum, &w.ProcessedAt); err != nil {
				return err
			}
			withdrawals = append(withdrawals, w)
		}
		return rows.Err()
	})

	return withdrawals, err
}

func (s *PostgresStorage) CreateSession(ctx context.Context, login, token string) error {
	query := `INSERT INTO sessions (token, user_login) VALUES ($1, $2)`
	_, err := s.db.ExecContext(ctx, query, token, login)
	return err
}

func (s *PostgresStorage) GetLoginByToken(ctx context.Context, token string) (string, error) {
	var login string
	query := `SELECT user_login FROM sessions WHERE token = $1`
	err := s.db.QueryRowContext(ctx, query, token).Scan(&login)
	return login, err
}
