package metadata

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestDiscoverPostgres(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("criando sqlmock: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(`SELECT tablename`).
		WillReturnRows(sqlmock.NewRows([]string{"tablename"}).AddRow("users").AddRow("orders"))

	mock.ExpectQuery(`SELECT\s+a\.attname`).
		WithArgs("users").
		WillReturnRows(sqlmock.NewRows([]string{"attname", "format_type", "is_nullable", "default_value", "is_primary_key", "is_auto_increment"}).
			AddRow("id", "integer", false, "nextval('users_id_seq'::regclass)", true, true).
			AddRow("name", "text", true, nil, false, false))
	mock.ExpectQuery(`FROM information_schema\.table_constraints`).
		WithArgs("users").
		WillReturnRows(sqlmock.NewRows([]string{"table_name", "column_name", "referenced_table_name", "referenced_column_name"}))

	mock.ExpectQuery(`SELECT\s+a\.attname`).
		WithArgs("orders").
		WillReturnRows(sqlmock.NewRows([]string{"attname", "format_type", "is_nullable", "default_value", "is_primary_key", "is_auto_increment"}).
			AddRow("id", "integer", false, "nextval('orders_id_seq'::regclass)", true, true).
			AddRow("user_id", "integer", false, nil, false, false).
			AddRow("total", "integer", false, nil, false, false))
	mock.ExpectQuery(`FROM information_schema\.table_constraints`).
		WithArgs("orders").
		WillReturnRows(sqlmock.NewRows([]string{"table_name", "column_name", "referenced_table_name", "referenced_column_name"}).
			AddRow("orders", "user_id", "users", "id"))

	catalog, err := Discover(context.Background(), db, "postgres")
	if err != nil {
		t.Fatalf("descobrindo metadata postgres: %v", err)
	}

	users, ok := catalog.Table("users")
	if !ok {
		t.Fatal("tabela users não encontrada")
	}
	if len(users.Columns) != 2 || !users.Columns[0].PrimaryKey || !users.Columns[0].AutoIncrement {
		t.Fatalf("metadata de users inesperada: %+v", users.Columns)
	}

	orders, ok := catalog.Table("orders")
	if !ok {
		t.Fatal("tabela orders não encontrada")
	}
	if len(orders.Columns) != 3 {
		t.Fatalf("metadata de orders inesperada: %+v", orders.Columns)
	}
	if len(catalog.ForeignKeys) != 1 {
		t.Fatalf("esperava 1 foreign key, obtive %+v", catalog.ForeignKeys)
	}
	if catalog.ForeignKeys[0].Table != "orders" || catalog.ForeignKeys[0].RefTable != "users" {
		t.Fatalf("foreign key inesperada: %+v", catalog.ForeignKeys[0])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectativas não atendidas: %v", err)
	}
}

func TestDiscoverMySQL(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("criando sqlmock: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(`FROM information_schema\.tables`).
		WillReturnRows(sqlmock.NewRows([]string{"table_name"}).AddRow("users").AddRow("orders"))

	mock.ExpectQuery(`FROM information_schema\.columns`).
		WithArgs("users").
		WillReturnRows(sqlmock.NewRows([]string{"column_name", "column_type", "is_nullable", "column_key", "extra"}).
			AddRow("id", "int(11)", "NO", "PRI", "auto_increment").
			AddRow("name", "varchar(255)", "YES", "", ""))
	mock.ExpectQuery(`FROM information_schema\.key_column_usage`).
		WithArgs("users").
		WillReturnRows(sqlmock.NewRows([]string{"table_name", "column_name", "referenced_table_name", "referenced_column_name"}))

	mock.ExpectQuery(`FROM information_schema\.columns`).
		WithArgs("orders").
		WillReturnRows(sqlmock.NewRows([]string{"column_name", "column_type", "is_nullable", "column_key", "extra"}).
			AddRow("id", "int(11)", "NO", "PRI", "auto_increment").
			AddRow("user_id", "int(11)", "NO", "", "").
			AddRow("total", "int(11)", "NO", "", ""))
	mock.ExpectQuery(`FROM information_schema\.key_column_usage`).
		WithArgs("orders").
		WillReturnRows(sqlmock.NewRows([]string{"table_name", "column_name", "referenced_table_name", "referenced_column_name"}).
			AddRow("orders", "user_id", "users", "id"))

	catalog, err := Discover(context.Background(), db, "mysql")
	if err != nil {
		t.Fatalf("descobrindo metadata mysql: %v", err)
	}

	users, ok := catalog.Table("users")
	if !ok {
		t.Fatal("tabela users não encontrada")
	}
	if len(users.Columns) != 2 || !users.Columns[0].PrimaryKey || !users.Columns[0].AutoIncrement {
		t.Fatalf("metadata de users inesperada: %+v", users.Columns)
	}

	if len(catalog.ForeignKeys) != 1 {
		t.Fatalf("esperava 1 foreign key, obtive %+v", catalog.ForeignKeys)
	}
	if catalog.ForeignKeys[0].Column != "user_id" || catalog.ForeignKeys[0].RefColumn != "id" {
		t.Fatalf("foreign key inesperada: %+v", catalog.ForeignKeys[0])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectativas não atendidas: %v", err)
	}
}
