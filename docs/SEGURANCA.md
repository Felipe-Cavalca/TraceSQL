# Segurança e dependências

Este documento explica como o TraceSQL cuida das bibliotecas de terceiros que usa e como reportar uma vulnerabilidade.

O TraceSQL abre conexões com bancos de dados e lê dados reais. Por isso, uma biblioteca vulnerável é um risco real. A regra do projeto é manter as dependências atualizadas e sob vigilância.

Veja também: [README](../README.md), [SECURITY.md](SECURITY.md) e [ROADMAP](../ROADMAP.md).

## Política de dependências

1. Toda dependência nova precisa de um motivo claro no PR.
2. Preferimos bibliotecas mantidas por organizações ou por várias pessoas.
3. Dependências diretas devem ser poucas. Uma biblioteca que arrasta muitas outras precisa de justificativa.
4. As atualizações chegam por PR automático (Dependabot) e passam pelo CI antes do merge.
5. Nenhum PR é mesclado com o CI falhando.

## Dependabot de versão

O Dependabot é um serviço do GitHub. Ele abre PRs quando uma dependência tem versão nova.

O projeto já recebe PRs do Dependabot de **segurança**. Esses PRs aparecem só quando existe uma falha conhecida. Faltava a atualização de **versão**, que mantém tudo em dia mesmo sem falha conhecida.

A configuração fica em `.github/dependabot.yml`.

### O que ele vigia

| Ecossistema | O que atualiza | Pasta |
| --- | --- | --- |
| `gomod` | Módulos Go do `go.mod` e do `go.sum`. | `/` |
| `github-actions` | Actions usadas nos arquivos de `.github/workflows/`. | `/` |

### Quando ele roda

- Uma vez por semana, às segundas-feiras.
- Fuso horário: `America/Sao_Paulo`.

### Como agrupa os PRs

- Atualizações **minor** e **patch** de um mesmo ecossistema vêm juntas em **um único PR**. Exemplo: um PR "grupo gomod minor e patch" com cinco módulos.
- Atualizações **major** vêm em PRs **separados**, um por dependência. Uma major pode quebrar a compatibilidade e merece revisão própria.
- No máximo 5 PRs abertos por ecossistema.

### Labels

Cada PR recebe a label `dependencies`. Ela já existe em `.github/labels.yml`.

Os PRs do Dependabot **não** recebem a label `upgrade`. Essa label marca quebra de compatibilidade e faz o fluxo de release subir a versão major do TraceSQL.

### Título do commit

Os PRs usam o prefixo `chore(deps)`. Exemplo: `chore(deps): bump github.com/spf13/cobra from 1.8.0 to 1.9.1`.

### Sem Release automática

No fluxo de release (`.github/workflows/tag_on_merge.yml`), a label `dependencies` gera uma versão **minor** nova. Sem ajuste, cada PR do Dependabot mesclado geraria uma Release.

Por isso, o `tag_on_merge.yml` ignora PRs abertos pelo Dependabot:

- O job que cria a tag e o rascunho de release só roda quando o autor do PR não é `dependabot[bot]`. A condição é `github.event.pull_request.user.login != 'dependabot[bot]'`.
- Os jobs que dependem dele também não rodam.
- O merge de um PR do Dependabot não cria tag nem Release.
- A atualização entra na próxima Release, criada por um PR normal (de uma pessoa).

### Como revisar um PR do Dependabot

1. Confira se o CI passou.
2. Leia a nota de versão que o Dependabot coloca no PR.
3. Se for major, procure mudanças que quebram a compatibilidade.
4. Faça o merge. Se algo quebrar, comente `@dependabot rebase` ou feche o PR.

### Limite conhecido

O GitHub dá ao Dependabot um token somente de leitura. O job `notify` do `ci.yml` comenta no PR e precisa de escrita. Por isso, em PRs do Dependabot esse comentário falharia, sem afetar o resultado dos testes.

## Reportar uma vulnerabilidade

NOTA: não abra uma issue pública para uma falha de segurança.

1. Abra a aba **Security** do repositório no GitHub.
2. Clique em **Report a vulnerability**.
3. Descreva o problema, os passos para reproduzir e a versão afetada.

O relato fica privado entre você e a pessoa que mantém o projeto. Ela responde pelo mesmo canal.

O projeto não usa e-mail para esse fim.

### Para quem administra o repositório

AVISO: o botão **Report a vulnerability** só aparece depois que um administrador ativa o *Private vulnerability reporting*.

1. Abra **Settings** do repositório.
2. Entre em **Code security**.
3. Em *Private vulnerability reporting*, clique em **Enable**.

O arquivo [SECURITY.md](SECURITY.md) é a política de segurança que o GitHub mostra na aba **Security**.
