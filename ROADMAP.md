# Roadmap do TraceSQL

Este arquivo concentra a visão de evolução do projeto, as fases planejadas e o contexto técnico que orienta as próximas entregas.

## Visão geral

O TraceSQL começou como um utilitário de terminal multiplataforma para exportação de dados relacionais. A direção de produto continua sendo transformar essa base em uma biblioteca reutilizável e, depois, em uma experiência integrada no VS Code.

## Fases do projeto

### Fase 1: CLI de exportação
- Entrega de um binário único para Windows, Linux e macOS.
- Implementação em Go 1.22+ para facilitar portabilidade e distribuição.
- Entrada por `.env`, flags e prompts interativos.
- Exportação do registro inicial e do grafo relacionado em um arquivo `.sql`.

### Fase 2: Biblioteca reutilizável
- Extrair a lógica de descoberta de relações e geração de SQL para um pacote reutilizável.
- Permitir reaproveitamento da regra de exportação em outras interfaces além do CLI.

### Fase 3: Extensão VS Code
- Criar uma interface visual para disparar exportações sem sair do editor.
- Consumir a biblioteca central em vez de duplicar regras de negócio.

### Fase 4: Publicação automatizada
- Consolidar a distribuição por releases.
- Automatizar build e empacotamento dos binários para múltiplas plataformas.

## Requisitos funcionais
- Conexão com MySQL, PostgreSQL e SQLite.
- Leitura de credenciais e opções via `.env` para reduzir prompts repetitivos.
- Descoberta de relações por foreign key nos bancos suportados.
- Geração de SQL para recriar schema e dados exportados, inclusive com dialeto de saída diferente do banco de origem.

## Stack técnica
- Linguagem: Go.
- Drivers: `pgx` (PostgreSQL), `go-sql-driver/mysql` e `glebarez/sqlite`.
- CLI: `spf13/cobra`.
- Configuração: `.env` com `godotenv` e complementação por flags.
- Build: `go build -trimpath -ldflags="-s -w" ./cmd/tracesql`.

## Testes
- Testes unitários com `go test ./...`.
- Cobertura voltada para configuração, prompts, metadata e exportação.
- Evolução desejada: ampliar testes de integração com bancos reais ou containers dedicados.

## Publicação
- O workflow `Release` é disparado por `repository_dispatch`.
- O evento usa `type: tag-created` com `client_payload.tag` apontando para a tag publicada.
- A automação gera binários para Linux, macOS e Windows em `amd64` e `arm64`.

## Estrutura atual

```text
cmd/tracesql/main.go      entrada do comando Cobra
internal/config/          leitura de env e flags
internal/db/              abertura de conexão por driver
internal/export/          geração do SQL exportado
internal/metadata/        descoberta de tabelas, colunas e FKs
internal/prompt/          perguntas interativas
configs/.env.example      template de configuração
```

## Próximos passos
- Refinar a experiência do CLI e a clareza das mensagens de erro.
- Continuar evoluindo a exportação de relações complexas e cenários entre dialetos.
- Fortalecer a cobertura de integração.
- Preparar a extração da lógica central para uma biblioteca reaproveitável.
