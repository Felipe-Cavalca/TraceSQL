# TraceSQL

Suite de ferramentas para inspecionar e exportar dados relacionais (MySQL, PostgreSQL e bancos em arquivo). A visão de longo prazo inclui uma extensão VS Code; começamos por um utilitário de terminal multiplataforma.

## Fase 1 – CLI de exportação
- Alvo: Windows, Linux e macOS, distribuído como artefato na página de Releases do GitHub.
- Entrada: conexão via variáveis de ambiente (.env) ou parâmetros no prompt.
- Fluxo: perguntar tabela origem, perguntar coluna de referência (padrão `id` se omitir), mapear tabelas relacionadas (referências e referenciadas) e exportar todos os registros ligados.
- Saída: arquivo `.sql` que insere todos os dados exportados. Começa preservando IDs; no futuro, suportará geração de novos IDs.

## Roadmap resumido
1) Script CLI de exportação (atual).  
2) Bibliotecar lógica de relacionamento/export para reuso.  
3) Extensão VS Code consumindo a biblioteca.  
4) Automatizar publicações em Release.

## Requisitos funcionais
- Conexão com MySQL, PostgreSQL e SQLite.
- Respeitar credenciais vindas de `.env` para evitar prompts repetitivos.
- Descobrir relações (FKs) para trazer tabelas que referenciam ou são referenciadas.
- Gerar SQL de insert idempotente para recriar dados exportados.

## Suite de testes recomendada
- Linguagem sugerida para a CLI: Python 3.11+ (portabilidade e drivers maduros).
- Testes unitários: `pytest` com `coverage` para parsing, conexão e geração de SQL.
- Testes de integração por banco: `pytest` + `docker-compose` subindo MySQL, PostgreSQL e SQLite (local). Usar marcadores `@pytest.mark.mysql`/`postgres`/`sqlite`.
- Testes end-to-end: invocar o CLI com `.env` de exemplo e validar o arquivo `.sql` gerado.
- Matriz CI: GitHub Actions com jobs paralelos por SO (ubuntu-22.04, windows-2022, macos-14) e serviços Docker para MySQL/Postgres.
- Linters/qualidade: `ruff` (lint+fmt) e `mypy` para contratos de conexão e modelos de metadados.

## Estrutura inicial sugerida
```
src/
  tracesql/
    cli.py           # entrada do comando
    exporters.py     # geração de INSERTs
    metadata.py      # descoberta de FKs e relações
    settings.py      # leitura de .env / args
tests/
  unit/
  integration/
  e2e/
.env.example
docker-compose.yml   # serviços MySQL/PostgreSQL para integração
```

## Próximos passos
- Confirmar linguagem (assumida Python) e criar esqueleto `src/` e `tests/`.
- Adicionar `.env.example` com variáveis de conexão.
- Configurar GitHub Actions com matriz de SO + bancos em serviços.
- Publicar primeiro binário/zip na Release após o MVP do CLI.
