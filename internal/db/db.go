package db

import (
	"database/sql"
	"fmt"
	"strings"

	// imports necessários para registrar os drivers
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

// Connect abre a conexão de acordo com o driver e devolve uma função para fechar.
func Connect(driver, dsn string) (*sql.DB, func(), error) {
	driver = strings.ToLower(driver)

	switch driver {
	case "mysql":
		// mysql driver registrado em init()
	case "postgres":
		driver = "pgx"
	case "sqlite":
		driver = "sqlite"
	default:
		return nil, func() {}, fmt.Errorf("driver não suportado: %s", driver)
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, func() {}, err
	}

	closeFn := func() {
		_ = db.Close()
	}
	return db, closeFn, nil
}
