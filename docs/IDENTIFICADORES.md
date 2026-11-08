# Como os nomes de tabelas e colunas são escritos no SQL

Veja também: [Como os valores viram literais SQL](EXPORTACAO.md).

## Por que isso importa

O arquivo `.sql` gerado precisa falar da mesma tabela e da mesma coluna que existem no banco de origem. Para isso, o TraceSQL coloca todo nome de tabela e de coluna entre aspas.

Um nome pode conter o próprio caractere de aspas. Exemplo: uma coluna chamada `a"b`. Dentro das aspas, esse caractere precisa ser escrito duas vezes. É a regra do SQL.

### O erro que esta regra corrige

Antes, o TraceSQL apagava o `"` e o `` ` `` do nome. A coluna `a"b` virava `ab`. O `.sql` apontava para outro nome, e a restauração falhava ou usava a coluna errada.

Agora o caractere é duplicado e o nome fica intacto.

## Exemplo

| Nome no banco | Dialeto de saída | Como sai no SQL |
|---|---|---|
| `users` | postgres | `"users"` |
| `users` | mysql | `` `users` `` |
| `a"b` | postgres | `"a""b"` |
| `a"b` | mysql | `` `a"b` `` |
| ``a`b`` | mysql | ``` `a``b` ``` |
| ``a`b`` | postgres | ``"a`b"`` |

Ao ler o SQL de volta, o banco desfaz a duplicação e encontra o nome original.

## Referência

Função: `quoteIdent(driver, ident string) string`, no pacote `internal/export`. A assinatura não muda.

- `driver` é o dialeto. Ele é normalizado como no resto do programa: `postgresql` e `pg` valem `postgres`; `sqlite3` vale `sqlite`; maiúsculas e minúsculas não importam (`MySQL` vale `mysql`).
- `ident` é o nome de uma tabela ou de uma coluna, exatamente como veio do banco.
- O retorno é o nome entre aspas, pronto para entrar no SQL.
- A função não retorna erro e não entra em pânico, qualquer que seja o nome.

### Regras

1. **Caractere de aspas do dialeto:**
   - `postgres` e `sqlite`: aspas duplas `"`.
   - `mysql`: crase `` ` ``.
   - Dialeto desconhecido ou vazio: usa a regra do `sqlite` (aspas duplas).
2. **Escape:** cada ocorrência do caractere de aspas do dialeto dentro do nome vira duas ocorrências. O outro tipo de aspas não é tocado.
3. **Nada mais é alterado.** O nome não é aparado, não muda de caixa e não perde caracteres. Espaços, pontos, acentos, emojis, barras, `;`, `'` e `--` ficam como estão.
4. **Nome vazio** (`""`) devolve vazio, sem aspas. Nomes vindos do catálogo não são vazios; o caso existe para a função não quebrar.
5. **Ponto no nome não separa nada.** `schema.tabela` é um único nome e sai inteiro entre aspas (`"schema.tabela"`). O TraceSQL não escreve nomes qualificados com esquema. Quem precisar de `schema.tabela` não deve esperar que o ponto seja tratado.

### Onde a regra vale

Em todo lugar onde o programa escreve um nome de tabela ou coluna no SQL:

- `CREATE TABLE` (nome da tabela, das colunas e das chaves estrangeiras);
- `INSERT` (tabela e lista de colunas);
- as tabelas de mapeamento do `--new-ids` (nomes fixos `old_id`, `new_id` e o nome da tabela de mapa);
- as consultas `SELECT * FROM <tabela> WHERE <coluna> = ?` que o programa faz no banco de origem. Nelas o dialeto é o do banco de origem.

Uma lista de colunas é escrita com cada nome escapado em separado e unida por `, `.

## Decisões

1. **SQLite usa aspas duplas**, como diz a issue #64. Antes, o SQLite recebia crase (que o SQLite aceita por compatibilidade). Agora recebe `"`, que é o padrão do SQL. A mudança só altera como o nome é escrito, não o nome.
2. **Dialeto desconhecido ou vazio usa a regra do sqlite**, igual ao que a doc de [valores](EXPORTACAO.md) já diz. Antes, esses casos recebiam crase.
3. **Nome vazio continua devolvendo vazio.** É o comportamento atual e fora do escopo desta correção.
4. **Sem tratamento de `schema.tabela`.** O código não divide o nome no ponto. Mudar isso seria uma funcionalidade nova.
5. **Só o caractere de aspas do dialeto é duplicado.** Duplicar o outro seria alterar o nome.

## Limitações conhecidas

- O tratamento da barra invertida em texto no MySQL (issue #63) é outro assunto e não está aqui.
- A função só trata aspas. Ela não valida se o nome é aceito pelo banco de destino (limite de tamanho, caracteres proibidos em alguns bancos).

## Como os testes devem conferir

Chame `quoteIdent(driver, nome)` e compare com a saída exata. Teste dentro do pacote `export` (a função não é exportada). Use apenas a biblioteca padrão.

Nas colunas, `(vazio)` é a string vazia. Crase aparece como `` ` ``.

### Escape por dialeto

| Nome do caso | Dialeto | Entrada | Saída |
|---|---|---|---|
| `simples_postgres` | `postgres` | `users` | `"users"` |
| `simples_mysql` | `mysql` | `users` | `` `users` `` |
| `simples_sqlite` | `sqlite` | `users` | `"users"` |
| `aspa_dupla_postgres` | `postgres` | `a"b` | `"a""b"` |
| `aspa_dupla_sqlite` | `sqlite` | `a"b` | `"a""b"` |
| `aspa_dupla_mysql_preservada` | `mysql` | `a"b` | `` `a"b` `` |
| `crase_mysql` | `mysql` | ``a`b`` | ``` `a``b` ``` |
| `crase_postgres_preservada` | `postgres` | ``a`b`` | ``"a`b"`` |
| `crase_sqlite_preservada` | `sqlite` | ``a`b`` | ``"a`b"`` |
| `varias_aspas_postgres` | `postgres` | `a"b"c` | `"a""b""c"` |
| `varias_crases_mysql` | `mysql` | ``a`b`c`` | ``` `a``b``c` ``` |
| `so_aspa_postgres` | `postgres` | `"` | `""""` |
| `so_crase_mysql` | `mysql` | `` ` `` | ``` ```` ``` ``` |
| `aspas_seguidas_postgres` | `postgres` | `""` | `""""""` |
| `aspa_no_inicio_e_fim_postgres` | `postgres` | `"x"` | `"""x"""` |
| `crase_no_inicio_e_fim_mysql` | `mysql` | ``` `x` ``` | ``` ```x``` ``` |
| `os_dois_tipos_postgres` | `postgres` | ``a"b`c`` | ``"a""b`c"`` |
| `os_dois_tipos_mysql` | `mysql` | ``a"b`c`` | ``` `a"b``c` ``` |
| `os_dois_tipos_sqlite` | `sqlite` | ``a"b`c`` | ``"a""b`c"`` |

Observação para o caso `so_crase_mysql`: a entrada é uma crase. A saída é crase, duas crases e crase, ou seja, quatro crases seguidas. O caso `so_aspa_postgres` é igual: entrada `"`, saída com quatro aspas duplas seguidas.

### Nome do dialeto

| Nome do caso | Dialeto | Entrada | Saída |
|---|---|---|---|
| `dialeto_pg` | `pg` | `a"b` | `"a""b"` |
| `dialeto_postgresql` | `postgresql` | `a"b` | `"a""b"` |
| `dialeto_postgres_maiusculo` | `POSTGRES` | `a"b` | `"a""b"` |
| `dialeto_sqlite3` | `sqlite3` | `a"b` | `"a""b"` |
| `dialeto_mysql_maiusculo` | `MySQL` | ``a`b`` | ``` `a``b` ``` |
| `dialeto_desconhecido` | `oracle` | `a"b` | `"a""b"` |
| `dialeto_desconhecido_crase_preservada` | `oracle` | ``a`b`` | ``"a`b"`` |
| `dialeto_vazio` | (vazio) | `a"b` | `"a""b"` |
| `dialeto_vazio_nome_simples` | (vazio) | `users` | `"users"` |

### Nomes sem aspas (nada muda)

Repetir nos dialetos `postgres`, `mysql` e `sqlite`. A saída é a entrada envolvida pelo caractere do dialeto, sem outra alteração. Abaixo, em `postgres`:

| Nome do caso | Entrada | Saída |
|---|---|---|
| `espaco` | `my table` | `"my table"` |
| `espacos_nas_pontas` | ` x ` | `" x "` |
| `ponto_nao_separa` | `public.users` | `"public.users"` |
| `unicode` | `usuário_ção` | `"usuário_ção"` |
| `emoji` | `tab😀` | `"tab😀"` |
| `apostrofo` | `o'brien` | `"o'brien"` |
| `ponto_e_virgula` | `a;b` | `"a;b"` |
| `comentario_sql` | `a--b` | `"a--b"` |
| `barra_invertida` | `a\b` | `"a\b"` |
| `caixa_preservada` | `UserID` | `"UserID"` |

Em `mysql` as mesmas entradas saem com crase (`` `my table` ``, `` `public.users` `` e assim por diante). Em `sqlite`, saem como em `postgres`.

### Nome vazio

| Nome do caso | Dialeto | Entrada | Saída |
|---|---|---|---|
| `vazio_postgres` | `postgres` | (vazio) | (vazio) |
| `vazio_mysql` | `mysql` | (vazio) | (vazio) |
| `vazio_sqlite` | `sqlite` | (vazio) | (vazio) |
| `vazio_dialeto_vazio` | (vazio) | (vazio) | (vazio) |

### O nome não é alterado (desfazer o escape)

Para cada caso das tabelas de escape, desfazer o escape do resultado tem de devolver a entrada. Isto é: tire o primeiro e o último caractere e troque cada par do caractere de aspas por um só. O resultado é igual a `Entrada`. Isso prova que o nome não foi alterado.

### Integração (SQLite em memória)

Estes casos conferem o ponto principal: o banco entende o nome escapado. Crie um banco SQLite em memória (os testes do pacote já têm um helper para isso).

1. Crie a tabela usando o resultado de `quoteIdent("sqlite", `t"x`)` como nome e `quoteIdent("sqlite", `c"y`)` como coluna: `CREATE TABLE <tabela> (<coluna> TEXT)`.
2. Insira uma linha e leia de volta com `SELECT <coluna> FROM <tabela>`. O valor deve voltar.
3. Consulte `sqlite_master` e confira que o nome da tabela guardado é exatamente `t"x`, e que `pragma table_info` lista a coluna `c"y`.
4. Repita com uma tabela de nome ``t`x`` e coluna ``c`y`` usando o dialeto `sqlite` (a crase não é escapada e o nome deve se manter).

### Exportação ponta a ponta

Exporte um registro de uma tabela `t"x` com coluna `c"y` (e uma chave estrangeira para outra tabela com aspas no nome) para o dialeto `sqlite`. Confira que o `.sql` contém `"t""x"` e `"c""y"`, que não contém `"tx"` nem `"cy"`, e que executar o `.sql` num banco SQLite novo recria a tabela com o nome original.

### Bytes e caracteres incomuns

Nada além do caractere de aspas do dialeto é alterado. Isto vale também para texto que não é UTF-8 válido, byte nulo e quebra de linha. A função não entra em pânico.

| Nome do caso | Dialeto | Entrada | Saída |
|---|---|---|---|
| `utf8_invalido_postgres` | `postgres` | `a` + byte `0xFF` + `b` | `"` + `a` + byte `0xFF` + `b` + `"` |
| `byte_nulo_mysql` | `mysql` | `a` + byte `0x00` + `b` | `` ` `` + `a` + byte `0x00` + `b` + `` ` `` |
| `quebra_de_linha_sqlite` | `sqlite` | `a`, quebra de linha, `b` | `"` + `a`, quebra de linha, `b` + `"` |

## Catálogo do SQLite

Quando o banco de origem é SQLite, o TraceSQL lê o esquema com `PRAGMA table_info` e `PRAGMA foreign_key_list` (pacote `internal/metadata`). Nessas consultas o nome da tabela também vai entre aspas duplas, e o `"` do nome é duplicado.

Antes, o `"` era removido. Para uma tabela chamada `t"x`, o PRAGMA consultava a tabela `tx`, que não existe. O catálogo saía vazio, e a tabela não era exportada.

Agora o PRAGMA recebe `"t""x"` e devolve as colunas e as chaves estrangeiras da tabela `t"x`. Uma crase no nome não precisa de escape nesse caso, porque o delimitador é `"`.

Como testar: crie um SQLite em memória com as tabelas `t"x` (colunas `id` e `c"y`) e ``t`x``, rode a descoberta do esquema e confira que as duas tabelas aparecem, com as colunas e as chaves estrangeiras corretas e com os nomes originais.

## Tabela de mapa do `--new-ids`

Com `--new-ids`, o TraceSQL cria uma tabela temporária de mapeamento por tabela exportada. O nome dela é `__tracesql_map_<tabela>`, onde `<tabela>` é o nome da tabela original, assim:

1. Letras são passadas para minúsculas.
2. Letras, dígitos, `"` e crase ficam como estão.
3. Qualquer outro caractere (espaço, ponto, hífen e assim por diante) vira `_`.

Depois, o nome inteiro passa por `quoteIdent` como qualquer outro. Por isso o `"` ou a crase que veio do nome da tabela é duplicado ao escrever.

| Nome do caso | Tabela | Dialeto | Nome da tabela de mapa no SQL |
|---|---|---|---|
| `mapa_simples` | `users` | `sqlite` | `"__tracesql_map_users"` |
| `mapa_aspas_duplas` | `t"x` | `sqlite` | `"__tracesql_map_t""x"` |
| `mapa_crase_sqlite` | ``t`x`` | `sqlite` | ``"__tracesql_map_t`x"`` |
| `mapa_crase_mysql` | ``t`x`` | `mysql` | ``` `__tracesql_map_t``x` ``` |
| `mapa_espaco` | `my table` | `sqlite` | `"__tracesql_map_my_table"` |
| `mapa_ponto` | `a.b` | `sqlite` | `"__tracesql_map_a_b"` |

Como testar: exporte um registro com `--new-ids` de uma tabela com cada nome acima (dialeto de saída `sqlite`) e confira que o `.sql` contém `CREATE TEMP TABLE <nome do mapa>` e um `INSERT INTO <nome do mapa> ("old_id", "new_id")`.
