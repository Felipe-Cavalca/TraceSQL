package config

import (
	"errors"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// LoadEnv carrega variáveis do arquivo informado, se existir.
func LoadEnv(path string) error {
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); err != nil {
		return nil // silencioso quando não existe
	}
	return godotenv.Load(path)
}

func ValidateDriver(d string) error {
	switch strings.ToLower(d) {
	case "mysql", "postgres", "sqlite":
		return nil
	default:
		return errors.New("use mysql | postgres | sqlite")
	}
}
