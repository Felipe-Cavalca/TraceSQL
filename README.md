# TraceSQL

Suite de ferramentas para inspecionar e exportar dados relacionais (MySQL, PostgreSQL e bancos em arquivo). A visão de longo prazo inclui uma extensão VS Code; começamos por um utilitário de terminal multiplataforma.

## Fase 1 – CLI de exportação
- Alvo: Windows, Linux e macOS, distribuído como binário único nas Releases do GitHub.
- Linguagem escolhida: Go 1.22+ (binário estático, sem runtime externo e boa portabilidade).
- Entrada: conexão via variáveis de ambiente (.env) ou parâmetros no prompt.
- Fluxo: perguntar tabela origem, perguntar coluna de referência (padrão `id` se omitir), mapear tabelas relacionadas (referências e referenciadas) e exportar todos os registros ligados.
- Saída: arquivo `.sql` que insere todos os dados exportados. Começa preservando IDs; no futuro, suportará geração de novos IDs.

## Roadmap resumido
1) CLI de exportação em Go (atual).
2) Bibliotecar lógica de relacionamento/export para reuso.
3) Extensão VS Code consumindo a biblioteca.
4) Automatizar publicações em Release.

## Requisitos funcionais
- Conexão com MySQL, PostgreSQL e SQLite.
- Respeitar credenciais vindas de `.env` para evitar prompts repetitivos.
- Descobrir relações (FKs) para trazer tabelas que referenciam ou são referenciadas.
- Gerar SQL de insert idempotente para recriar dados exportados.

## Stack técnica (Go)
- Drivers: `github.com/jackc/pgx/v5` (PostgreSQL), `github.com/go-sql-driver/mysql`, `modernc.org/sqlite` (CGO habilitado) ou `github.com/mattn/go-sqlite3` dentro do devcontainer.
- CLI: `spf13/cobra` para comandos e prompts simples.
- Configuração: `.env` lido com `github.com/joho/godotenv` + flags.
- Build: `go build -trimpath -ldflags="-s -w" ./cmd/tracesql` gerando binário estático.
- Distribuição: anexar binários (linux-amd64, linux-arm64, windows-amd64, darwin-amd64/arm64) nas Releases.

## Testes
- Unitários e integração: `go test ./...` (usar `-tags=integration` quando subir bancos de teste via Docker Compose).
- End-to-end: invocar o CLI com `.env` de exemplo e validar o arquivo `.sql` gerado.
- Qualidade: `golangci-lint run` (lint+fmt) e `staticcheck` opcional.
- Matriz CI: GitHub Actions por SO (ubuntu-22.04, windows-2022, macos-14) com serviços Docker para MySQL/Postgres.

## Estrutura inicial sugerida
```
cmd/
  tracesql/main.go     # entrada do comando
internal/export/       # geração de INSERTs
internal/metadata/     # descoberta de FKs e relações
internal/config/       # leitura de .env / args
internal/db/           # conexões e adapters por banco
configs/.env.example
configs/docker-compose.yml   # serviços MySQL/PostgreSQL para integração
```

## Ambiente de desenvolvimento
- Dev Container Go em `.devcontainer/` (Go 1.22, SQLite dev, clientes MySQL/Postgres, Docker socket montado para testes).
- Requisitos locais mínimos: Docker instalado para usar o devcontainer; fora dele, basta Go 1.22+ se não precisar rodar os bancos locais.

## Próximos passos
- Inicializar `go.mod` e esqueleto em `cmd/internal` conforme estrutura acima.
- Adicionar `.env.example` com variáveis de conexão.
- Criar `docker-compose.yml` com MySQL e PostgreSQL para testes.
- Configurar GitHub Actions com matriz de SO + bancos em serviços.
- Publicar primeiro binário/zip na Release após o MVP do CLI.
