package config

import (
	"os"
	"testing"
)

func TestNew(t *testing.T) {
	origRunAddress := os.Getenv("RUN_ADDRESS")
	origDatabaseURI := os.Getenv("DATABASE_URI")
	origAccrualAddress := os.Getenv("ACCRUAL_SYSTEM_ADDRESS")

	defer func() {
		os.Setenv("RUN_ADDRESS", origRunAddress)
		os.Setenv("DATABASE_URI", origDatabaseURI)
		os.Setenv("ACCRUAL_SYSTEM_ADDRESS", origAccrualAddress)
	}()

	t.Run("Default values", func(t *testing.T) {
		os.Unsetenv("RUN_ADDRESS")
		os.Unsetenv("DATABASE_URI")
		os.Unsetenv("ACCRUAL_SYSTEM_ADDRESS")

		os.Args = []string{"cmd"}

		cfg := New()
		if cfg.RunAddress != "localhost:8080" {
			t.Errorf("expected RunAddress 'localhost:8080', got '%s'", cfg.RunAddress)
		}
		if cfg.AccrualAddress != "http://localhost:8081" {
			t.Errorf("expected AccrualAddress 'http://localhost:8081', got '%s'", cfg.AccrualAddress)
		}
	})

	t.Run("Environment variables override defaults", func(t *testing.T) {
		os.Setenv("RUN_ADDRESS", "192.168.1.1:9090")
		os.Setenv("DATABASE_URI", "postgres://test:test@localhost:5432/test")
		os.Setenv("ACCRUAL_SYSTEM_ADDRESS", "http://accrual:8081")

		os.Args = []string{"cmd"}

		cfg := New()
		if cfg.RunAddress != "192.168.1.1:9090" {
			t.Errorf("expected RunAddress '192.168.1.1:9090', got '%s'", cfg.RunAddress)
		}
		if cfg.DatabaseURI != "postgres://test:test@localhost:5432/test" {
			t.Errorf("expected DatabaseURI 'postgres://test...', got '%s'", cfg.DatabaseURI)
		}
	})
}
