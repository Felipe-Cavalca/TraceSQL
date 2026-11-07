# Segurança e dependências

Este documento explica como o TraceSQL cuida das bibliotecas de terceiros que usa e como reportar uma vulnerabilidade.

O TraceSQL abre conexões com bancos de dados e lê dados reais. Por isso, uma biblioteca vulnerável é um risco real. A regra do projeto é manter as dependências atualizadas e sob vigilância.

Veja também: [README](../README.md), [SECURITY.md](SECURITY.md) e [ROADMAP](../ROADMAP.md).

O CI também procura vulnerabilidades (`govulncheck`) e usa a mesma versão do Go que o projeto declara.

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

O GitHub dá ao Dependabot um token somente de leitura. O job `notify` do `ci.yml` comenta no PR e precisa de escrita. Por isso, o `notify` não roda em PRs do Dependabot (veja "Job notify e Dependabot").

## govulncheck

O `govulncheck` é a ferramenta oficial do time do Go. Ela compara o seu projeto com o banco de vulnerabilidades do Go (`vuln.go.dev`).

Ela só avisa quando o código **chama de fato** a função vulnerável. Isso evita alarmes falsos de uma biblioteca que está no `go.mod` mas cuja parte com falha o projeto não usa.

Ela olha duas coisas:

- as bibliotecas de terceiros do `go.mod`;
- a biblioteca padrão do Go, da versão usada na compilação.

### Quando roda

| Gatilho | Onde |
| --- | --- |
| Push na `main` e nas branches `codex/**` | Job `vulncheck` do `.github/workflows/ci.yml`. |
| Pull request para a `main` | Job `vulncheck` do `.github/workflows/ci.yml`. |
| Toda segunda-feira, às 06:00 UTC | Gatilho `schedule` (`cron: '0 6 * * 1'`) do mesmo `ci.yml`. |

A execução semanal existe porque novas vulnerabilidades surgem sem que o código mude.

Na execução semanal, os jobs `test` e `notify` **não** rodam. Só o `vulncheck` roda. Para isso, `test` e `notify` têm a condição `github.event_name != 'schedule'`.

### O que o job faz

1. Faz o checkout do código.
2. Instala o Go indicado no `go.mod` (`go-version-file: go.mod`), na versão de correção mais recente da mesma linha. A biblioteca padrão do Go também recebe correções de segurança. O dev deve conferir se `check-latest: true` garante isso.
3. Roda `govulncheck ./...` na raiz do repositório.

Resultado:

- Nenhuma vulnerabilidade alcançável: o job passa.
- Qualquer vulnerabilidade alcançável: o job falha e o PR não deve ser mesclado.

Na execução semanal, o job que falha aparece na aba **Actions** e o GitHub avisa por e-mail. Nenhuma issue é aberta sozinha.

### Versão do govulncheck

O `ci.yml` usa uma versão fixa do govulncheck, na linha de comando: `go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...`. A versão fixa garante que o resultado não muda sozinho. Para atualizá-la, edite o `ci.yml`.

A `v1.9.0` exige Go 1.26, a versão do `go.mod`.

### Como reproduzir na sua máquina

1. Instale o Go indicado no `go.mod`.
2. Rode `go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...`.

### O que fazer quando o job falha

1. Leia o relatório. Ele mostra o identificador (`GO-AAAA-NNNN`), a função chamada e a versão que corrige.
2. Se a falha está em uma dependência, atualize-a para a versão corrigida e rode `go mod tidy`.
3. Se a falha está na biblioteca padrão, o job já usa o Go mais recente da linha. Se a correção só existe em uma linha nova do Go, suba a versão `go` do `go.mod`.
4. Se não existe correção, abra uma issue com o identificador. Decida em conjunto se a função vulnerável pode ser evitada.

## Versão do Go

O projeto usa uma só versão do Go: a do `go.mod`, hoje `go 1.26.0`.

Antes, o `ci.yml` e o `release.yml` usavam Go `1.22` fixo, e o Dev Container também. Os testes e os binários eram feitos em uma versão diferente da que o projeto declara.

Agora:

- O `go.mod` declara `go 1.26.0`. A versão sobe porque o `modernc.org/sqlite` mais recente e o `govulncheck` mais recente exigem Go 1.26.
- O `ci.yml` e o `release.yml` leem a versão do `go.mod` (`go-version-file: go.mod`).
- O `.devcontainer/Dockerfile` usa a imagem de Dev Container Go 1.26 (por exemplo, `mcr.microsoft.com/devcontainers/go:1-1.26-bookworm`; o dev confirma o nome da tag).
- O README cita Go 1.26+.

Quando o `go.mod` mudar, o CI e a Release acompanham. O Dev Container precisa ser atualizado à mão.

## Job notify e Dependabot

O job `notify` comenta o resultado em cada pull request. O Dependabot tem token somente de leitura e não consegue comentar.

Por isso, o `notify` tem a condição `github.actor != 'dependabot[bot]'` além da atual. Nos PRs do Dependabot ele não roda e não falha.

## Actions fixadas por SHA

No `ci.yml`, cada action é referenciada pelo SHA completo do commit (40 caracteres), e não por uma tag como `v4`. O motivo: uma tag pode ser movida para outro código, e um commit não.

Cada linha traz a versão em comentário. Exemplo:

```yaml
- uses: actions/checkout@<sha-de-40-caracteres> # v4.2.2
```

O Dependabot, ao atualizar uma action, troca o SHA e o comentário juntos.

Regras:

- O formato é `uses: <dono>/<action>@<40 caracteres hexadecimais> # v<versão>`.
- O SHA deve pertencer à versão escrita no comentário.
- Não use `@main` nem tag sem SHA.
- Por enquanto vale só para as actions do `ci.yml`. Os outros workflows continuam por tag.

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
