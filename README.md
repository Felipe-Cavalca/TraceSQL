# TraceSQL

Suite de ferramentas para inspecionar e exportar dados relacionais (MySQL, PostgreSQL e bancos em arquivo). A visão de longo prazo inclui uma extensão VS Code; começamos por um utilitário de terminal multiplataforma.

## Fase 1 – CLI de exportação
- Alvo: Windows, Linux e macOS, distribuído como binário único nas Releases do GitHub.
- Linguagem escolhida: Go 1.22+ (binário estático, sem runtime externo e boa portabilidade).
- Entrada: conexão via variáveis de ambiente (.env) ou parâmetros no prompt. Pergunta interativamente se algo estiver faltando.
- Fluxo atual: escolher banco/tabela, coluna de referência (padrão `id`), registro a exportar e se os IDs devem ser preservados ou regenerados. Gera um `.sql` com o `INSERT` correspondente.
- Saída: arquivo `.sql` nomeado como `export_<tabela>_<registro>.sql` (ou caminho informado via flag/ENV).

## Roadmap resumido
1) CLI de exportação em Go (atual).
2) Bibliotecar lógica de relacionamento/export para reuso.
3) Extensão VS Code consumindo a biblioteca.
4) Automatizar publicações em Release.

## Requisitos funcionais
- Conexão com MySQL, PostgreSQL e SQLite.
- Respeitar credenciais vindas de `.env` para evitar prompts repetitivos.
- (Em progresso) Descobrir relações (FKs) para trazer tabelas relacionadas.
- Gerar SQL de insert idempotente para recriar dados exportados, com opção de preservar IDs ou deixar o banco gerar novos.

## Stack técnica (Go)
- Drivers: `pgx` (PostgreSQL), `go-sql-driver/mysql` e `glebarez/sqlite` (sem CGO).
- CLI: `spf13/cobra` para comandos e prompts simples.
- Configuração: `.env` lido com `godotenv` + flags.
- Build: `go build -trimpath -ldflags="-s -w" ./cmd/tracesql` gerando binário estático.
- Distribuição: workflow de Release compila e anexa binários (linux-amd64/arm64, windows-amd64/arm64, darwin-amd64/arm64).

## Testes
- Unitários: `go test ./...` (CI roda em matriz ubuntu/macos/windows).
- Integração planejada: usar Docker Compose para MySQL/Postgres quando a lógica de relacionamento for adicionada.

## Estrutura atual
```
cmd/tracesql/main.go      # entrada do comando Cobra
internal/config/          # leitura/env/flags
internal/db/              # abertura de conexão por driver
internal/export/          # geração de INSERT simples
internal/metadata/        # placeholder para descoberta de FKs
internal/prompt/          # perguntas interativas
configs/.env.example      # template de configuração
```

## Uso rápido
1) Preencha `configs/.env.example` e salve como `.env` (ou exporte as variáveis):  
   - `TRACESQL_DRIVER` (postgres | mysql | sqlite)  
   - `TRACESQL_DSN` (ex.: `postgres://user:pass@localhost:5432/db`)  
   - `TRACESQL_NEW_IDS` (`true` para omitir a coluna de referência no INSERT)  
   - `TRACESQL_OUT` (opcional: caminho do arquivo de saída)  
2) Rode `go run ./cmd/tracesql` ou o binário baixado. Campos ausentes (tabela/coluna/registro) serão perguntados no terminal.  
3) O dump sai em `export_<tabela>_<registro>.sql` (ou caminho passado via `--out`).  

## Ambiente de desenvolvimento
- Dev Container Go em `.devcontainer/` (Go 1.22, SQLite dev, clientes MySQL/Postgres, Docker socket montado para testes).
- Requisitos locais mínimos: Docker instalado para usar o devcontainer; fora dele, basta Go 1.22+ se não precisar rodar os bancos locais.

## Próximos passos
- Evoluir exportação para incluir relações (FKs) e múltiplas tabelas.
- Adicionar testes de integração com Docker Compose.
- Incluir lint/format (golangci-lint) na CI.
