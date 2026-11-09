# Relações por nome (`--relations-by-name`)

Este documento explica como o TraceSQL descobre relações pelo nome das colunas, quando isso funciona e quando falha.

Veja também: [README](../README.md) e [EXPORTACAO](EXPORTACAO.md).

## Resumo

- O modo é uma **heurística**. Ele adivinha relações pelo nome da coluna. Ele não prova que a relação existe.
- Ele só vale para bancos que **não declaram** a foreign key. Foreign keys reais sempre têm prioridade.
- Ele pode **deixar relações de fora** (grafo incompleto) e pode **inventar uma relação errada** (grafo com linhas a mais).
- Foreign keys reais compostas (várias colunas) são suportadas: o TraceSQL as lê do banco, segue a relação usando todas as colunas da chave e as recria na saída. Já a inferência por nome continua só de uma coluna: ela nunca monta uma relação composta. Veja a [issue #32](https://github.com/Felipe-Cavalca/TraceSQL/issues/32).

## Como a regra funciona

A inferência roda depois que o TraceSQL lê o esquema do banco e aplica `--ignore-table-suffix`. Só as tabelas que sobraram participam. Para cada tabela e cada coluna:

1. **A coluna termina em `_id`?** A comparação ignora maiúsculas e minúsculas. `user_id` e `USER_ID` servem. `id`, `userid`, `userId` e `user_code` não servem.
2. **A coluna já tem uma foreign key real?** Se sim, o TraceSQL não faz nada. A relação real fica como está.
3. **Tira o `_id` do fim.** O que sobra é o "nome base". Em `user_id` o nome base é `user`. Em `billing_address_id` é `billing_address`.
4. **Procura a tabela de destino** com o nome base, nesta ordem:
   1. **Nome exato** (sem diferenciar maiúsculas): a tabela se chama igual ao nome base. `user_id` acha a tabela `user`. `users_id` acha a tabela `users`.
   2. **Nome no plural:** o nome base é o singular de uma tabela. As duas únicas formas de singular são:
      - tabela termina em `ies`: troca `ies` por `y` (`categories` vira `category`);
      - tabela termina em `s`: tira o `s` (`users` vira `user`).
      
      O plural só é aceito se **exatamente uma** tabela resultar nesse nome. Se duas tabelas disputam o mesmo nome, nenhuma é escolhida.
5. **A tabela de destino tem uma coluna `id` que é chave primária?** Se não tiver, a relação é descartada. (Chave primária composta que inclui uma coluna `id` também passa.)
6. **Cria a relação:** `tabela.coluna_id` aponta para `destino.id`. O nome da coluna de destino é sempre `id`, em minúsculas.

O nome exato vence o plural. Se existirem as tabelas `user` e `users`, a coluna `user_id` aponta para `user`.

### O que o TraceSQL não olha

- O **tipo** das colunas. `user_id` texto apontando para `users.id` inteiro é aceito.
- Os **dados**. Ele não confere se os valores existem na tabela de destino.
- O **resto do nome**. Prefixos, sufixos e palavras no meio não são entendidos.

### O que muda na exportação

As relações inferidas entram no grafo como se fossem foreign keys reais:

- Elas guiam a busca de linhas pais e filhas (respeitando `--depth`).
- Elas aparecem como `FOREIGN KEY (...) REFERENCES ...` no `CREATE TABLE` do arquivo gerado, quando a tabela de destino também foi exportada.
- Elas funcionam com `--new-ids`, que reescreve as referências.

Com `--log`, o TraceSQL informa quantas relações foram inferidas.

## O que funciona e o que não funciona

Em todos os exemplos, a tabela de destino tem `id` como chave primária e a coluna de origem não tem foreign key declarada.

| Caso | Tabelas e coluna | Resultado | Motivo |
| --- | --- | --- | --- |
| Padrão básico | `orders.user_id`, tabela `users` | Funciona: `orders.user_id -> users.id` | Plural com `s` |
| Tabela no singular | `orders.user_id`, tabela `user` | Funciona | Nome exato |
| Plural em `ies` | `products.category_id`, tabela `categories` | Funciona | `categories` vira `category` |
| Maiúsculas | `ORDERS.USER_ID`, tabela `Users` | Funciona | Comparação sem diferenciar maiúsculas |
| Coluna no plural | `orders.users_id`, tabela `users` | Funciona | Nome exato |
| Nome base com `_` | `orders.order_item_id`, tabela `order_items` | Funciona | `order_items` vira `order_item` |
| Auto-referência pelo nome da própria tabela | `categories.category_id`, tabela `categories` | Funciona: `categories.category_id -> categories.id` | A tabela de destino é a própria tabela |
| Auto-referência com outro nome | `categories.parent_id` | **Não funciona** | Não existe tabela `parent` |
| Plural diferente de `s` ou `ies` | `orders.address_id`, tabela `addresses` | **Não funciona** | `addresses` vira `addresse`, não `address` |
| Plural irregular | `orders.person_id`, tabela `people` | **Não funciona** | Não há regra para plural irregular |
| Singular termina em `s` | `orders.status_id`, tabela `statuses` | **Não funciona** | `statuses` vira `statuse`. Funciona se a tabela se chamar `status` (nome exato) |
| Prefixo na coluna | `orders.billing_address_id`, tabela `addresses` | **Não funciona** | O nome base `billing_address` não existe |
| Prefixo na tabela | `orders.user_id`, tabela `app_users` | **Não funciona** | O nome base `user` não bate com `app_users` |
| Sufixo diferente de `_id` | `orders.user_code`, `orders.userid`, `orders.user_fk` | **Não funciona** | A coluna não termina em `_id` |
| camelCase | `orders.userId`, tabela `users` | **Não funciona** | Não termina em `_id` com sublinhado |
| Tabela inexistente | `orders.coupon_id`, sem tabela `coupon` ou `coupons` | Ignorada, sem erro | Não há destino |
| Tabela de destino ignorada | `orders.user_id`, tabela `users` removida por `--ignore-table-suffix` | Ignorada | Só tabelas que sobraram participam |
| Destino sem `id` | `orders.user_id`, `users` com chave primária `user_id` | **Não funciona** | O destino precisa ter chave primária chamada `id` |
| Destino com `id` sem ser chave | `orders.user_id`, `users.id` sem `PRIMARY KEY` | **Não funciona** | O destino precisa ter chave primária chamada `id` |
| Disputa entre tabelas (plural) | `x.baby_id`, tabelas `babys` e `babies` | Ignorada (ambígua) | As duas viram `baby` e nenhuma é exata |
| Nome exato e plural juntos | `x.user_id`, tabelas `user` e `users` | Aponta para `user` | O nome exato vence |
| Coluna já tem FK real | `orders.user_id -> customers.id` declarada | Mantém a real, não infere `users` | FK real tem prioridade |
| FK composta | `(a_id, b_id)` apontando para `t(a, b)` | **Não é inferida** (só vale se o banco declarar) | A heurística olha uma coluna por vez |
| Falso positivo | `audit.user_id` guarda o id de outro sistema, e existe a tabela `users` | Relação criada, **errada** | O nome basta, não há checagem de dados nem de tipo |
| Falso positivo (auto-relação não pretendida) | `employees.employee_id` com outro significado (por exemplo, matrícula) | Relação criada, **errada**: `employees.employee_id -> employees.id` | Coluna `[própria tabela]_id` aponta para a própria tabela |
| Coluna só `_id` | coluna chamada `_id` | Ignorada | Nome base vazio |

### Sobre foreign keys compostas

Quando o banco declara uma FK composta, cada coluna dela já conta como "com relação" e a heurística a pula. Quando o banco **não** declara, as colunas são avaliadas uma a uma, como colunas simples. Isso pode criar relações para colunas individuais que, na verdade, só fazem sentido juntas. O TraceSQL nunca monta uma relação de duas ou mais colunas por nome.

## O que fazer quando o grafo sai incompleto ou errado

1. **Grafo incompleto (falso negativo).** Declare a foreign key no banco, mesmo que seja só em uma cópia de homologação. É a forma mais segura. Foreign keys reais valem sempre, inclusive com nomes fora do padrão, plural irregular e prefixos.
2. **Não dá para mexer no banco.** Renomeie a tabela ou a coluna no banco usado na exportação para seguir o padrão (`[tabela]_id` com tabela no plural simples ou no nome exato).
3. **Grafo com linhas demais (falso positivo).** Use `--ignore-table-suffix` para tirar tabelas do grafo, e `--depth` para limitar quantos níveis são percorridos. Se o problema for a própria inferência, rode sem `--relations-by-name` e use só as foreign keys reais.
4. **Sempre confira o resultado.** Rode com `--log` e veja quantas relações foram inferidas. Abra o `.sql` e procure as linhas `FOREIGN KEY` que você não esperava.
5. **Chaves compostas.** O TraceSQL não infere. Declare a FK composta no banco; as declaradas são suportadas.

## Casos de teste (para quem escreve os testes)

Os testes ficam em `internal/metadata/metadata_test.go` e usam o método `Catalog.WithNameInferredForeignKeys()`. Ele recebe o catálogo e devolve o catálogo novo e a quantidade de relações **adicionadas** (as foreign keys que já existiam não contam).

Notação: `T(col*, col, ...)` é uma tabela. A coluna com `*` é chave primária. `FK` é uma foreign key já declarada, no formato `tabela.coluna -> destino.coluna`. `esperado` lista só as relações **adicionadas**. A lista completa de relações do catálogo é as `FK` declaradas mais as adicionadas.

**Atenção ao montar o `Catalog` à mão nos testes unitários.** O catálogo guarda as tabelas por chave em minúsculas (`users`), enquanto `Table.Name` mantém a caixa original (`Users`). Um `Catalog` montado com chave de caixa mista (por exemplo `"Users"`) não infere nada nos testes unitários. Isso é só um limite de como o teste monta o catálogo. No uso real, o catálogo vem de `Discover` e a inferência ignora maiúsculas e minúsculas. Por isso o caso 4 (`Users` e `USER_ID`) também é coberto de ponta a ponta no exportador.

Todos os casos devem rodar também no nível do exportador (`internal/export`, com SQLite em memória e `RelationsByName: true`) para pelo menos os casos 1, 4, 5, 14 e 21: o arquivo `.sql` deve conter, ou não, o `FOREIGN KEY` e as linhas relacionadas.

### Casos que devem criar relação

| # | Nome | Esquema | Esperado |
| --- | --- | --- | --- |
| 1 | plural com s | `users(id*)`, `orders(id*, user_id)` | `orders.user_id -> users.id` |
| 2 | singular exato | `user(id*)`, `orders(id*, user_id)` | `orders.user_id -> user.id` |
| 3 | plural em ies | `categories(id*)`, `products(id*, category_id)` | `products.category_id -> categories.id` |
| 4 | maiúsculas | `Users(id*)`, `Orders(id*, USER_ID)` | `Orders.USER_ID -> Users.id` |
| 5 | coluna no plural | `users(id*)`, `orders(id*, users_id)` | `orders.users_id -> users.id` |
| 6 | nome base com underscore | `order_items(id*)`, `shipments(id*, order_item_id)` | `shipments.order_item_id -> order_items.id` |
| 7 | auto-referência pelo nome da tabela | `categories(id*, category_id)` | `categories.category_id -> categories.id` |
| 8 | várias colunas na mesma tabela | `users(id*)`, `posts(id*, user_id, author_id)`, `authors(id*)` | `posts.user_id -> users.id` e `posts.author_id -> authors.id` (2 adicionadas) |
| 9 | exato vence plural | `user(id*)`, `users(id*)`, `orders(id*, user_id)` | `orders.user_id -> user.id` (1 adicionada) |
| 10 | chave primária composta com id | `users(id*, tenant*)`, `orders(id*, user_id)` | `orders.user_id -> users.id` |
| 11 | tipos diferentes são aceitos | `users(id* INTEGER)`, `orders(id*, user_id TEXT)` | `orders.user_id -> users.id` |

### Casos que NÃO devem criar relação (falso negativo documentado)

| # | Nome | Esquema | Esperado |
| --- | --- | --- | --- |
| 12 | plural em es | `addresses(id*)`, `orders(id*, address_id)` | nenhuma (0) |
| 13 | plural irregular | `people(id*)`, `orders(id*, person_id)` | nenhuma |
| 14 | prefixo na coluna | `addresses(id*)`, `orders(id*, billing_address_id)` | nenhuma |
| 15 | prefixo na tabela | `app_users(id*)`, `orders(id*, user_id)` | nenhuma |
| 16 | auto-referência com outro nome | `categories(id*, parent_id)` | nenhuma |
| 17 | singular terminado em s com tabela no plural | `statuses(id*)`, `orders(id*, status_id)` | nenhuma |
| 18 | sufixo diferente | `users(id*)`, `orders(id*, user_code, userid, user_fk)` | nenhuma |
| 19 | camelCase | `users(id*)`, `orders(id*, userId)` | nenhuma |
| 20 | coluna `id` sozinha | `users(id*)`, `orders(id*)` | nenhuma (nenhuma coluna `id` vira relação) |
| 21 | tabela inexistente | `users(id*)`, `orders(id*, coupon_id)` | nenhuma e sem erro |
| 22 | destino sem coluna id | `users(user_id*)`, `orders(id*, user_id)` | nenhuma |
| 23 | destino com id fora da chave | `users(id, name*)`, `orders(id*, user_id)` | nenhuma |
| 24 | ambiguidade entre plurais | `babys(id*)`, `babies(id*)`, `x(id*, baby_id)` | nenhuma |
| 25 | coluna `_id` sozinha | `users(id*)`, `x(id*, _id)` | nenhuma |
| 26 | FK composta não declarada | `parent(a*, b*)`, `child(id*, a_id, b_id)` | nenhuma (sem tabelas `a` nem `b`) |
| 27 | catálogo vazio | nenhuma tabela | nenhuma, devolve o mesmo catálogo e 0 |

### Casos de FK existente e idempotência

| # | Nome | Esquema | Esperado |
| --- | --- | --- | --- |
| 28 | FK real tem prioridade | `users(id*)`, `customers(id*)`, `orders(id*, user_id)`, FK `orders.user_id -> customers.id` | 0 adicionadas. Continua só `orders.user_id -> customers.id` |
| 29 | FK real igual à inferida | `users(id*)`, `orders(id*, user_id)`, FK `orders.user_id -> users.id` | 0 adicionadas. Sem duplicata |
| 30 | FK composta declarada | `parent(a*, b*)`, `child(id*, a_id, b_id)`, FK composta `child(a_id, b_id) -> parent(a, b)` | 0 adicionadas |
| 31 | rodar duas vezes | qualquer esquema do caso 1 | a segunda chamada devolve 0 adicionadas e a mesma lista |
| 32 | ordem do resultado | caso 8 | lista ordenada por tabela, coluna, destino e coluna de destino |

### Falsos positivos (a heurística erra, e o teste registra isso)

| # | Nome | Esquema | Esperado |
| --- | --- | --- | --- |
| 33 | id de outro sistema | `users(id*)`, `audit(id*, user_id)` | `audit.user_id -> users.id`, mesmo que `user_id` não seja desta base |
| 34 | auto-relação não pretendida | `employees(id*, employee_id)` | `employees.employee_id -> employees.id` |

### Casos com `--ignore-table-suffix` (nível do exportador)

| # | Nome | Esquema e opções | Esperado |
| --- | --- | --- | --- |
| 35 | destino ignorado | `user_log(id*)`, `orders(id*, user_log_id)` com sufixo ignorado `_log` | nenhuma relação; o `.sql` não tem `user_log` nem `FOREIGN KEY` para ele |
| 36 | tabela de origem ignorada | `orders_log(id*, user_id)` ignorada pelo sufixo `_log` | sem relação; `orders_log` não aparece no `.sql` |
| 37 | flag desligada | caso 1 com `RelationsByName: false` | o grafo não inclui `orders` ao partir de `users`; o `.sql` não tem `FOREIGN KEY` |
| 38 | flag ligada ponta a ponta | caso 1 com dados (`users` com id 1 e 2; `orders` com uma linha de `user_id` 1 e uma de `user_id` 2) e `RelationsByName: true` partindo de `users.id = 1` | o `.sql` contém o `CREATE TABLE` de `orders` com `FOREIGN KEY ("user_id") REFERENCES "users" ("id")` e o `INSERT` da linha de `orders` com `user_id = 1` (e não o da linha com `user_id = 2`) |

Se a regra mudar (por exemplo, passar a aceitar `addresses` ou prefixos), os casos 12, 14, 15 e 17 viram casos positivos. Atualize este documento junto com o teste.
