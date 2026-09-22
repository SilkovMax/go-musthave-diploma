package config

import (
	"flag"
	"os"
)

type Config struct {
	RunAddress     string
	DatabaseURI    string
	AccrualAddress string
}

func New() *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.RunAddress, "a", "", "адрес и порт запуска сервиса")
	flag.StringVar(&cfg.DatabaseURI, "d", "", "адрес подключения к базе данных")
	flag.StringVar(&cfg.AccrualAddress, "r", "", "адрес системы расчёта начислений")

	flag.Parse()

	if cfg.RunAddress == "" {
		if env := os.Getenv("RUN_ADDRESS"); env != "" {
			cfg.RunAddress = env
		} else {
			cfg.RunAddress = "localhost:8080"
		}
	}

	if cfg.DatabaseURI == "" {
		if env := os.Getenv("DATABASE_URI"); env != "" {
			cfg.DatabaseURI = env
		} else {
			cfg.DatabaseURI = "postgres://postgres:123456@localhost:5432/gophermart?sslmode=disable"
		}
	}

	if cfg.AccrualAddress == "" {
		if env := os.Getenv("ACCRUAL_SYSTEM_ADDRESS"); env != "" {
			cfg.AccrualAddress = env
		} else {
			cfg.AccrualAddress = "http://localhost:8081"
		}
	}

	return cfg
}
