//go:build integration

package export

// itMotores lista os motores usados pela matriz. Com a tag `integration` a
// matriz é completa (3 origens x 3 destinos); Postgres e MySQL só rodam quando
// TRACESQL_TEST_PG_DSN e TRACESQL_TEST_MYSQL_DSN estão definidas (senão, t.Skip).
func itMotores() []itMotor {
	return []itMotor{itSQLite, itPostgres, itMySQL}
}
