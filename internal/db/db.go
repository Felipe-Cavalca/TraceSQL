package db

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/glebarez/sqlite"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Open retorna uma conex?o database/sql configurada para o driver informado.
func Open(driver, dsn string) (*sql.DB, error) {
	switch strings.ToLower(driver) {
	case "postgres", "postgresql", "pg":
		return sql.Open("pgx", dsn)
	case "mysql":
		return sql.Open("mysql", dsn)
	case "sqlite", "sqlite3":
		return sql.Open("sqlite", dsn)
	default:
		return nil, fmt.Errorf("driver n?o suportado: %s", driver)
	}
}
