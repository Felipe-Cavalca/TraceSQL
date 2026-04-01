# TraceSQL

`TraceSQL` é um CLI em Go para exportar um registro e o seu grafo relacional para um arquivo `.sql`. Ele conecta em bancos `postgres`, `mysql` ou `sqlite`, descobre relacionamentos por chave estrangeira, gera o schema das tabelas envolvidas e escreve os `INSERT`s no dialeto de saída escolhido.

O histórico das fases do projeto e os próximos passos agora ficam em [ROADMAP.md](ROADMAP.md).

## O que o projeto faz
- Exporta um registro base a partir de `tabela`, `coluna` e `valor`.
- Descobre relações pai/filho via foreign keys e inclui os registros conectados.
- Gera `CREATE TABLE IF NOT EXISTS` e `INSERT`s no dialeto `postgres`, `mysql` ou `sqlite`.
- Permite manter os IDs originais ou regenerá-los quando a tabela usa chave primária auto increment.
- Aceita configuração por `.env`, flags e prompts interativos no terminal.

## Requisitos
- Para uso normal, baixe o binário da Release compatível com seu sistema.
- Go 1.22+ é necessário apenas se você quiser rodar o projeto a partir do código-fonte.
- Um DSN válido para o banco de origem.
- Banco de origem suportado: `postgres`, `mysql` ou `sqlite`.

## Como usar
1. Baixe na aba Releases o binário compatível com seu sistema. Os assets seguem o padrão `tracesql-<os>-<arch>`, por exemplo `tracesql-linux-amd64`, `tracesql-darwin-arm64` ou `tracesql-windows-amd64.exe`.
2. Em Linux ou macOS, dê permissão de execução ao arquivo com `chmod +x <binario>`.
3. Copie `configs/.env.example` para `.env` e ajuste os valores necessários, ou exporte as variáveis manualmente.
4. Execute o binário. Se algum dado não for informado, o CLI pergunta no terminal. Se a coluna não for passada, o padrão é `id`.
5. O arquivo será salvo como `export_<tabela>_<registro>.sql`, a menos que você informe `--out`.

### Exemplo com binário

```bash
./tracesql-linux-amd64 \
  --driver postgres \
  --dsn 'postgres://user:pass@localhost:5432/app' \
  --table orders \
  --column id \
  --record 10 \
  --output-driver mysql \
  --out ./tmp/orders_10.sql
```

### Exemplo interativo

```bash
./tracesql-linux-amd64
```

Se algum campo obrigatório não for informado por flag, o CLI pergunta no terminal:
- driver
- dsn
- tabela de origem
- coluna de referência
- valor do registro
- se deve gerar novos IDs

## Executando a partir do código-fonte

Se você estiver desenvolvendo no projeto, também pode rodar localmente com:

```bash
go run ./cmd/tracesql
```

## Flags disponíveis

| Flag | Obrigatória | Descrição |
| --- | --- | --- |
| `--driver` | Sim | Driver do banco de origem: `postgres`, `mysql` ou `sqlite`. |
| `--dsn` | Sim | String de conexão do banco de origem. |
| `--table` | Sim | Tabela inicial da exportação. |
| `--record` | Sim | Valor do registro que será usado como ponto de partida. |
| `--column` | Não | Coluna de referência usada no filtro inicial. Padrão: `id`. |
| `--output-driver` | Não | Dialeto SQL de saída. Padrão: mesmo driver da origem. |
| `--out` | Não | Caminho do arquivo `.sql` gerado. |
| `--new-ids` | Não | Omite a chave de referência dos `INSERT`s para gerar novos IDs quando suportado. |
| `--log` | Não | Escreve logs de execução no `stderr`. |

## Variáveis de ambiente suportadas

O projeto carrega automaticamente um arquivo `.env` na raiz do repositório.

| Variável | Descrição |
| --- | --- |
| `TRACESQL_DRIVER` | Mesmo valor da flag `--driver`. |
| `TRACESQL_DSN` | Mesmo valor da flag `--dsn`. |
| `TRACESQL_OUTPUT_DRIVER` | Mesmo valor da flag `--output-driver`. |
| `TRACESQL_NEW_IDS` | Mesmo valor da flag `--new-ids`. Aceita `true`, `1`, `yes`, `sim` e equivalentes. |
| `TRACESQL_OUT` | Mesmo valor da flag `--out`. |
| `TRACESQL_LOG` | Mesmo valor da flag `--log`. |

## Saída gerada
- Nome padrão: `export_<tabela>_<registro>.sql`.
- Conteúdo: `CREATE TABLE IF NOT EXISTS` seguido dos `INSERT`s das linhas exportadas.
- Quando `--new-ids` está ativo, o TraceSQL cria mapeamentos temporários para preservar referências entre tabelas relacionadas.

## Desenvolvimento
- Dev Container em `.devcontainer/` com Go 1.22, SQLite, clientes MySQL/Postgres e Docker socket para testes.
- Testes automatizados: `go test ./...`.
