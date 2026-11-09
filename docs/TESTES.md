# Testes de integração com bancos reais

Os testes unitários conferem o texto do SQL gerado. Estes testes provam que o banco aceita o SQL e que os dados restaurados são os mesmos.

Fluxo de cada caso: cria a origem (SQL nativo) -> `export.Run` -> restaura o `.sql` num banco novo do destino -> compara schema e dados.

Onde fica: `internal/export/*_integration_test.go` (pacote `export`, sem dependência nova).

## O que roda e quando

| Comando | O que roda |
| --- | --- |
| `go test ./...` | Só SQLite em memória (`modernc.org/sqlite`), sempre. |
| `go test -tags integration ./...` | Matriz 3x3 (SQLite, Postgres, MySQL, origem -> destino, inclusive mesmo dialeto). Postgres e MySQL são pulados (`t.Skip`) se o DSN não estiver definido. |

DSNs de administração (o teste cria e apaga um banco `tsql_it_*` por caso):

- `TRACESQL_TEST_PG_DSN`, por exemplo `postgres://tracesql:tracesql@127.0.0.1:55432/postgres?sslmode=disable`
- `TRACESQL_TEST_MYSQL_DSN`, por exemplo `root:tracesql@tcp(127.0.0.1:53306)/`

## Rodando localmente

```bash
docker compose -f docker-compose.test.yml up -d --wait
export TRACESQL_TEST_PG_DSN='postgres://tracesql:tracesql@127.0.0.1:55432/postgres?sslmode=disable'
export TRACESQL_TEST_MYSQL_DSN='root:tracesql@tcp(127.0.0.1:53306)/'
go test -tags integration ./...
docker compose -f docker-compose.test.yml down
```

No CI, o job `integration` do `ci.yml` sobe Postgres e MySQL como service containers e roda `go test -tags integration`.

## Como ler os casos

- Os casos estão em `matriz_integration_test.go` (`itCasos`); a fixture, em `fixture_integration_test.go`: inteiros, decimais, booleanos, Unicode, data/hora, JSON, binário, NULL, grafo de loja, FK para a própria tabela, ciclo entre tabelas, `--new-ids`, barra invertida (#63) e nomes com aspas (#64).
- Comparação (`comparacao_integration_test.go`): schema normalizado (tabelas, colunas, nulabilidade, família do tipo, FKs) e dados linha a linha. Tipos que mudam de forma legítima entre dialetos (por exemplo boolean -> tinyint) ficam numa tabela de equivalências.

## Bugs conhecidos

Caso que expõe um bug real do exporter não muda a expectativa: ele é marcado no campo `bugs` do caso e pulado com `t.Skip("bug ...")`. Para rodar esses casos mesmo assim (ver se continuam falhando ou se foram corrigidos): `TRACESQL_TEST_RUN_KNOWN_BUGS=1 go test -tags integration ./internal/export`. Ao corrigir um bug, remova a entrada em `bugs`.
