package storage

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func setupMockDB(t *testing.T) (*PostgresStorage, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	return &PostgresStorage{db: db}, mock
}

func TestPostgresStorage_GetUserBalance(t *testing.T) {
	store, mock := setupMockDB(t)
	defer store.db.Close()

	t.Run("Success", func(t *testing.T) {
		mock.ExpectQuery(`SELECT COALESCE\(SUM\(bonus_amount\), 0\)`).
			WithArgs("testuser").
			WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(1000.0))

		mock.ExpectQuery(`SELECT COALESCE\(SUM\(sum\), 0\)`).
			WithArgs("testuser").
			WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(200.0))

		current, withdrawn, err := store.GetUserBalance(context.Background(), "testuser")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if current != 800.0 || withdrawn != 200.0 {
			t.Errorf("expected 800.0/200.0, got %v/%v", current, withdrawn)
		}
	})
}

func TestPostgresStorage_WithdrawBalance(t *testing.T) {
	store, mock := setupMockDB(t)
	defer store.db.Close()

	t.Run("Success Withdrawal", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec(`SELECT 1 FROM users WHERE login = \$1 FOR UPDATE`).
			WithArgs("testuser").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectQuery(`SELECT COALESCE\(SUM\(bonus_amount\), 0\)`).
			WithArgs("testuser").
			WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(1000.0))
		mock.ExpectQuery(`SELECT COALESCE\(SUM\(sum\), 0\)`).
			WithArgs("testuser").
			WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(200.0))
		mock.ExpectExec(`INSERT INTO withdrawals`).
			WithArgs("testuser", "12345678903", 300.0).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		err := store.WithdrawBalance(context.Background(), "testuser", "12345678903", 300.0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("Insufficient Funds", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec(`SELECT 1 FROM users WHERE login = \$1 FOR UPDATE`).
			WithArgs("testuser").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectQuery(`SELECT COALESCE\(SUM\(bonus_amount\), 0\)`).
			WithArgs("testuser").
			WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(100.0))
		mock.ExpectQuery(`SELECT COALESCE\(SUM\(sum\), 0\)`).
			WithArgs("testuser").
			WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(0.0))
		mock.ExpectRollback()

		err := store.WithdrawBalance(context.Background(), "testuser", "12345678903", 500.0)
		if !errors.Is(err, ErrInsufficientFunds) {
			t.Errorf("expected ErrInsufficientFunds, got %v", err)
		}
	})
}

func TestPostgresStorage_CreateUser(t *testing.T) {
	store, mock := setupMockDB(t)
	defer store.db.Close()

	t.Run("Success", func(t *testing.T) {
		mock.ExpectExec(`INSERT INTO users`).
			WithArgs("testuser", "hashed_pass").
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := store.CreateUser(context.Background(), "testuser", "hashed_pass")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestPostgresStorage_GetUser(t *testing.T) {
	store, mock := setupMockDB(t)
	defer store.db.Close()

	t.Run("Success", func(t *testing.T) {
		mock.ExpectQuery(`SELECT login, password_hash FROM users WHERE login = \$1`).
			WithArgs("testuser").
			WillReturnRows(sqlmock.NewRows([]string{"login", "password_hash"}).AddRow("testuser", "hashed_pass"))

		user, err := store.GetUser(context.Background(), "testuser")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Login != "testuser" {
			t.Errorf("expected login 'testuser', got '%s'", user.Login)
		}
	})

	t.Run("User not found", func(t *testing.T) {
		mock.ExpectQuery(`SELECT login, password_hash FROM users WHERE login = \$1`).
			WithArgs("nonexistent").
			WillReturnError(sql.ErrNoRows)

		_, err := store.GetUser(context.Background(), "nonexistent")
		if !errors.Is(err, ErrUserNotFound) {
			t.Errorf("expected ErrUserNotFound, got %v", err)
		}
	})
}

func TestPostgresStorage_AddOrder(t *testing.T) {
	store, mock := setupMockDB(t)
	defer store.db.Close()

	t.Run("Order already processed by same user", func(t *testing.T) {
		mock.ExpectQuery(`SELECT user_login FROM orders WHERE number = \$1`).
			WithArgs("123").
			WillReturnRows(sqlmock.NewRows([]string{"user_login"}).AddRow("testuser"))

		err := store.AddOrder(context.Background(), "testuser", "123")
		if !errors.Is(err, ErrOrderAlreadyProcessed) {
			t.Errorf("expected ErrOrderAlreadyProcessed, got %v", err)
		}
	})

	t.Run("New order success", func(t *testing.T) {
		mock.ExpectQuery(`SELECT user_login FROM orders WHERE number = \$1`).
			WithArgs("123").
			WillReturnError(sql.ErrNoRows)
		mock.ExpectExec(`INSERT INTO orders`).
			WithArgs("123", "testuser").
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := store.AddOrder(context.Background(), "testuser", "123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestPostgresStorage_GetOrdersByUser(t *testing.T) {
	store, mock := setupMockDB(t)
	defer store.db.Close()

	t.Run("Success", func(t *testing.T) {
		now := time.Now()
		mock.ExpectQuery(`SELECT number, status, bonus_amount, uploaded_at`).
			WithArgs("testuser").
			WillReturnRows(sqlmock.NewRows([]string{"number", "status", "bonus_amount", "uploaded_at"}).
				AddRow("123", "PROCESSED", 500.0, now))

		orders, err := store.GetOrdersByUser(context.Background(), "testuser")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(orders) != 1 {
			t.Errorf("expected 1 order, got %d", len(orders))
		}
	})
}

func TestPostgresStorage_GetOrdersForProcessing(t *testing.T) {
	store, mock := setupMockDB(t)
	defer store.db.Close()

	t.Run("Success", func(t *testing.T) {
		mock.ExpectQuery(`SELECT number, user_login, status, bonus_amount FROM orders WHERE status IN`).
			WillReturnRows(sqlmock.NewRows([]string{"number", "user_login", "status", "bonus_amount"}).
				AddRow("123", "testuser", "NEW", 0.0))

		orders, err := store.GetOrdersForProcessing(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(orders) != 1 {
			t.Errorf("expected 1 order, got %d", len(orders))
		}
	})
}

func TestPostgresStorage_UpdateOrderStatus(t *testing.T) {
	store, mock := setupMockDB(t)
	defer store.db.Close()

	t.Run("Success", func(t *testing.T) {
		accrual := 500.0
		mock.ExpectExec(`UPDATE orders SET status = \$1, bonus_amount = COALESCE\(\$2, bonus_amount\) WHERE number = \$3`).
			WithArgs("PROCESSED", &accrual, "123").
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := store.UpdateOrderStatus(context.Background(), "123", "PROCESSED", &accrual)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestPostgresStorage_GetWithdrawalsByUser(t *testing.T) {
	store, mock := setupMockDB(t)
	defer store.db.Close()

	t.Run("Success", func(t *testing.T) {
		now := time.Now()
		mock.ExpectQuery(`SELECT order_number, sum, processed_at FROM withdrawals WHERE user_login = \$1`).
			WithArgs("testuser").
			WillReturnRows(sqlmock.NewRows([]string{"order_number", "sum", "processed_at"}).
				AddRow("123", 500.0, now))

		withdrawals, err := store.GetWithdrawalsByUser(context.Background(), "testuser")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(withdrawals) != 1 {
			t.Errorf("expected 1 withdrawal, got %d", len(withdrawals))
		}
	})
}

func TestPostgresStorage_CreateSession(t *testing.T) {
	store, mock := setupMockDB(t)
	defer store.db.Close()

	t.Run("Success", func(t *testing.T) {
		mock.ExpectExec(`INSERT INTO sessions`).
			WithArgs("token123", "testuser").
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := store.CreateSession(context.Background(), "testuser", "token123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestPostgresStorage_GetLoginByToken(t *testing.T) {
	store, mock := setupMockDB(t)
	defer store.db.Close()

	t.Run("Success", func(t *testing.T) {
		mock.ExpectQuery(`SELECT user_login FROM sessions WHERE token = \$1`).
			WithArgs("token123").
			WillReturnRows(sqlmock.NewRows([]string{"user_login"}).AddRow("testuser"))

		login, err := store.GetLoginByToken(context.Background(), "token123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if login != "testuser" {
			t.Errorf("expected login 'testuser', got '%s'", login)
		}
	})
}
