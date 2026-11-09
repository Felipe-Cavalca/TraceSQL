//go:build !integration

package export

// itMotores lista os motores usados pela matriz. Sem a tag `integration` só o
// SQLite em memória participa (roda sempre no `go test ./...`).
func itMotores() []itMotor {
	return []itMotor{itSQLite}
}
