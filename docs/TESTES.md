# Testes de integração com bancos reais

Este documento explica como o TraceSQL é testado contra Postgres, MySQL e SQLite de verdade, o que cada teste confere e como rodar tudo na sua máquina e no CI.

Veja também: [README](../README.md), [ROADMAP](../ROADMAP.md), [EXPORTACAO](EXPORTACAO.md) (valores), [IDENTIFICADORES](IDENTIFICADORES.md) (nomes) e [RELACOES](RELACOES.md) (relações por nome).

## Por que existe

Os testes unitários conferem o texto do SQL gerado. Eles não provam que o banco aceita esse SQL, nem que os dados restaurados são os mesmos. A parte mais arriscada do TraceSQL é justamente essa: ler de um dialeto e escrever em outro (tipos, literais, nomes e chaves).

Esta suíte fecha a lacuna: ela cria dados num banco real, exporta com o TraceSQL, restaura o arquivo `.sql` num banco real vazio e compara o resultado com a origem. Se alguém mexer em `formatValue`, `quoteIdent` ou no mapeamento de tipos e o SQL deixar de funcionar, um destes testes falha.

Em resumo:

- **SQLite** roda sempre, junto com `go test ./...`. Não precisa de nada instalado.
- **Postgres** e **MySQL** rodam quando você informa onde está o banco (variável de ambiente). Sem isso, esses testes são pulados com uma mensagem clara.
- Não há dependência nova no `go.mod`. Os bancos sobem com `docker run` (local) ou service containers (CI).

## Começo rápido

Só SQLite (nada a instalar):

```bash
go test ./tests/integration/... -count=1
```

Tudo, com Postgres e MySQL em containers (PowerShell ou bash; detalhes em [Rodando localmente](#rodando-localmente)):

```bash
docker run --rm -d --name tracesql-it-pg    -e POSTGRES_PASSWORD=it      -p 55432:5432 postgres:16
docker run --rm -d --name tracesql-it-mysql -e MYSQL_ROOT_PASSWORD=it    -p 53306:3306 mysql:8
export TRACESQL_IT_POSTGRES_DSN='postgres://postgres:it@127.0.0.1:55432/postgres?sslmode=disable&timezone=UTC'
export TRACESQL_IT_MYSQL_DSN='root:it@tcp(127.0.0.1:53306)/mysql?charset=utf8mb4&multiStatements=true'
go test ./tests/integration/... -count=1 -v
```

## Como um teste funciona

Todo teste da matriz segue o mesmo fluxo:

1. **Preparar a origem.** Cria um banco novo no motor de origem e roda o DDL e os `INSERT`s da fixture, em SQL nativo desse motor.
2. **Exportar.** Chama `export.Run` com `Driver = origem` e `OutputDriver = destino`. O resultado é o texto do `.sql`.
3. **Restaurar.** Cria um banco novo e vazio no motor de destino e executa o texto inteiro, de uma vez, numa única conexão.
4. **Comparar.** Lê schema e dados dos dois bancos e confere a equivalência definida em [O que significa "igual"](#o-que-significa-igual).
5. **Limpar.** Remove os bancos criados (`DROP DATABASE`), mesmo se o teste falhar.

Duas observações práticas sobre o passo 3:

- O arquivo cria tabelas temporárias de mapeamento quando há `--new-ids`. Elas só existem na conexão que as criou, então o arquivo todo precisa rodar na **mesma conexão**. Use `db.Conn(ctx)` (Postgres e MySQL) ou `SetMaxOpenConns(1)` (SQLite).
- No MySQL o DSN de restauração precisa de `multiStatements=true`. No Postgres, `ExecContext` sem argumentos aceita vários comandos. No SQLite, `Exec` aceita vários comandos.

## A matriz origem para destino

São 3 origens e 3 destinos, 9 combinações. O destino é sempre o motor **real** do dialeto de saída:

| Destino (`--output-driver`) | Onde o `.sql` é restaurado |
| --- | --- |
| `postgres` | Postgres 16 real |
| `mysql` | MySQL 8 real |
| `sqlite` | SQLite em memória (`modernc.org/sqlite`, o mesmo driver do programa) |

| Origem \ Destino | postgres | mysql | sqlite |
| --- | --- | --- | --- |
| **postgres** | `pg_para_pg` | `pg_para_mysql` | `pg_para_sqlite` |
| **mysql** | `mysql_para_pg` | `mysql_para_mysql` | `mysql_para_sqlite` |
| **sqlite** | `sqlite_para_pg` | `sqlite_para_mysql` | `sqlite_para_sqlite` |

Os nomes da segunda linha são os nomes dos subtestes: `t.Run("<origem>_para_<destino>", ...)`. Origem e destino do mesmo motor usam bancos diferentes (origem e destino nunca são o mesmo banco).

Regras da matriz:

- Os cenários "completos" (fixture de tipos, grafo, ciclos, `--new-ids`, identificadores) rodam nas **9 combinações**.
- Os cenários de opções (`--depth`, `--relations-by-name`, `--ignore-table-suffix`) rodam só na **diagonal** (`pg_para_pg`, `mysql_para_mysql`, `sqlite_para_sqlite`). A opção não depende do dialeto de saída, então 3 execuções bastam.
- Uma combinação cujo motor não está disponível (sem DSN) é pulada com `t.Skip("TRACESQL_IT_POSTGRES_DSN não definido")`. O SQLite nunca é pulado.
- Com `TRACESQL_IT_REQUIRE=1`, DSN ausente **falha** o teste em vez de pular. O CI usa isso para não passar em silêncio sem rodar nada.
- MariaDB: motor extra, opcional. Com `TRACESQL_IT_MARIADB_DSN` definido, a suíte repete as combinações em que o MySQL aparece, usando o mesmo driver `mysql`, com os nomes `mariadb_para_*` e `*_para_mariadb`. Sem a variável, é pulado, e `TRACESQL_IT_REQUIRE` não o torna obrigatório.

## A fixture

### Tabela comum `tipos`

Uma tabela com um tipo de cada família, criada **nativamente** em cada origem. O TraceSQL lê o tipo que o motor informa, então a fixture não pode usar o mesmo DDL nos três.

| Coluna | postgres | mysql | sqlite | Família |
| --- | --- | --- | --- | --- |
| `id` | `bigint PRIMARY KEY` | `bigint PRIMARY KEY` | `INTEGER PRIMARY KEY` | int |
| `codigo` | `varchar(20) NOT NULL UNIQUE` | `varchar(20) NOT NULL UNIQUE` | `VARCHAR(20) NOT NULL UNIQUE` | texto |
| `lote` | `varchar(10) NOT NULL` | `varchar(10) NOT NULL` | `VARCHAR(10) NOT NULL` | texto |
| `c_int` | `integer` | `int` | `INTEGER` | int |
| `c_big` | `bigint` | `bigint` | `BIGINT` | int |
| `c_dec` | `numeric(12,2)` | `decimal(12,2)` | `NUMERIC(12,2)` | decimal |
| `c_float` | `double precision` | `double` | `REAL` | float |
| `c_bool` | `boolean` | `tinyint(1)` | `BOOLEAN` | bool |
| `c_char` | `varchar(40)` | `varchar(40)` | `VARCHAR(40)` | texto |
| `c_texto` | `text` | `longtext` | `TEXT` | texto |
| `c_bin` | `bytea` | `longblob` | `BLOB` | binário |
| `c_json` | `jsonb` | `json` | `JSON` | json |
| `c_data` | `date` | `date` | `DATE` | data |
| `c_hora` | `time` | `time` | `TIME` | hora |
| `c_ts` | `timestamp` | `datetime` | `TIMESTAMP` | timestamp |
| `c_ts_frac` | `timestamp(6)` | `datetime(6)` | `TIMESTAMP` | timestamp |

Todas as colunas, menos `id`, `codigo` e `lote`, aceitam `NULL`. Todas as linhas têm `lote = 'A'`. A exportação usa `Table: "tipos"`, `Column: "lote"`, `Record: "A"` e traz as 10 linhas de uma vez.

Os `id` vão de 101 a 110 de propósito: não coincidem com o `id` gerado por `--new-ids` (que começa em 1), então um id mantido por engano aparece no teste.

Valores. Coluna não citada na linha é `NULL`. Notação: `<LF>` `<CR>` `<TAB>` `<SUB>` (Ctrl-Z, `0x1A`) são um byte cada; `\` é uma barra invertida real; `<0x..>` são bytes.

| `id` | `codigo` | O que a linha testa | Valores |
| --- | --- | --- | --- |
| 101 | `r01-normal` | valores comuns | `c_int` 42, `c_big` 9000000000, `c_dec` 99.90, `c_float` 1.5, `c_bool` verdadeiro, `c_char` `abc`, `c_texto` `olá mundo`, `c_bin` `<0x48 0x69>`, `c_json` `{"a":1,"b":[true,null,"x"]}`, `c_data` 2024-05-01, `c_hora` 10:30:00, `c_ts` 2024-05-01 10:00:00, `c_ts_frac` 2024-05-01 10:00:00.123456 |
| 102 | `r02-extremos` | limites numéricos e de data | `c_int` -2147483648, `c_big` 9223372036854775807, `c_dec` 9999999999.99, `c_float` -0.25, `c_bool` falso, `c_char` 40 vezes `x`, `c_texto` 10000 vezes `z`, `c_bin` `<0x00 0xFF 0x27 0x5C 0x5C 0x00>`, `c_json` `[]`, `c_data` 1970-01-01, `c_hora` 00:00:00, `c_ts` 1970-01-01 00:00:00, `c_ts_frac` 1999-12-31 23:59:59.999999 |
| 103 | `r03-barras-aspas` | barra, aspa simples e dupla | `c_texto` `C:\temp\'x`, `c_char` `it's "q"`, `c_json` `{"p":"C:\\temp","q":"it's"}` (as duas barras são o escape do JSON, texto real) |
| 104 | `r04-injecao` | texto que tenta fechar o literal | `c_texto` `'; DROP TABLE tipos;--`, `c_char` `\'; DROP TABLE tipos;--` |
| 105 | `r05-unicode` | Unicode sem normalização | `c_texto` `ação ñ 日本語 😀 العربية`, `c_char` `cafe` seguido de U+0301 (acento combinante, forma decomposta) |
| 106 | `r06-controle` | controles aceitos pelos três motores | `c_texto` `l1<LF>l2<CR><LF>l3<TAB>tab<SUB>sub` |
| 107 | `r07-vazios` | vazio é diferente de nulo | `c_texto` `''`, `c_char` `''`, `c_bin` vazio (zero bytes), `c_json` `null` (o JSON nulo, não o `NULL` do banco) |
| 108 | `r08-todos-nulos` | tudo `NULL` | só `id`, `codigo` e `lote` |
| 109 | `r09-barra-n-escrito` | barra seguida de letra | `c_texto` `\n` (barra e `n`), `c_char` `\\` (duas barras), `c_json` `{"nl":"a\nb"}` (no JSON, barra e `n` formam o escape de quebra de linha) |
| 110 | `r10-zeros` | zero não é nulo nem falso | `c_int` 0, `c_dec` 0.00, `c_float` 0, `c_bool` `NULL`, `c_ts` 2000-02-29 12:00:00 |

NUL (`0x00`) em **texto** fica de fora desta tabela porque Postgres e SQLite (com `modernc`) não o aceitam. Ele tem teste próprio (veja o grupo `Nul*`).

### Tabelas extras por origem

Tipos que só existem em um motor ficam em tabelas próprias, que só rodam com aquela origem (3 destinos cada). Cada tabela tem `id bigint/INTEGER PRIMARY KEY`, `codigo` único e `lote = 'A'`.

| Tabela | Origem | Colunas (tipo nativo) e valores |
| --- | --- | --- |
| `extras_pg` | postgres | `c_interval interval` = `1 day 02:03:04`; `c_uuid uuid` = `3f2b8c3e-0000-4000-8000-000000000000`; `c_array integer[]` = `{1,2,3}`; `c_ponto point` = `(1,2)`; `c_tstz timestamptz` = `2024-05-01 10:00:00+02` (vale 08:00:00 UTC); `c_numlivre numeric` = `3.14159`; `c_bpchar character(5)` = `ab`. Uma segunda linha com tudo `NULL`. |
| `extras_mysql` | mysql | `c_flag tinyint(1)` = 1 (e uma linha com 0); `c_ano year` = 2024; `c_enum enum('verde','azul')` = `azul`. Uma linha com tudo `NULL`. |
| `sem_sinal` | mysql | `c_u32 int unsigned` = 4294967295; `c_u64 bigint unsigned` = 18446744073709551615. Fica em tabela própria porque pode falhar sozinha. |
| `extras_sqlite` | sqlite | `c_desconhecido MY_TYPE` = `abc`; `c_semtipo` (sem tipo declarado) = 42 numa linha e `x` em outra; `c_bool1 TINYINT(1)` = 1; `c_big_sem_sinal UNSIGNED BIG INT` = 7; `c_datahora DATETIME` = `2024-05-01T10:00:00Z` (com `T` e `Z`, como o SQLite aceita). Uma linha com tudo `NULL`. |

### Tabelas relacionais

Estas tabelas formam os grafos. Cada uma é criada com `FOREIGN KEY` declarada, na ordem em que o motor aceita. Ids com tamanhos diferentes (`9`, `10`, `100`) para pegar ordenação por texto.

**Grafo da loja** (`loja`):

| Tabela | Colunas | Linhas (`id`, `codigo`, resto) |
| --- | --- | --- |
| `categorias` | `id` PK, `codigo` único, `nome`, `pai_id` FK para `categorias.id` | (9, `cat-raiz`, `Raiz`, NULL), (10, `cat-filha`, `Filha`, 9), (100, `cat-neta`, `Neta`, 10) |
| `clientes` | `id` PK, `codigo` único, `nome` | (10, `c-ana`, `Ana`), (20, `c-bruno`, `Bruno`) |
| `produtos` | `id` PK, `codigo` único, `nome`, `preco` numeric(10,2), `categoria_id` FK para `categorias.id` | (1, `p-1`, `Caneta`, 3.50, 100), (2, `p-2`, `Caderno`, 12.00, 10) |
| `pedidos` | `id` PK, `codigo` único, `cliente_id` FK para `clientes.id`, `criado_em` timestamp | (1001, `ped-1`, 10, 2024-05-01 10:00:00), (1002, `ped-2`, 20, 2024-05-02 11:00:00) |
| `itens` | `id` PK, `pedido_id` FK para `pedidos.id`, `produto_id` FK para `produtos.id`, `quantidade` int | (1, 1001, 1, 2), (2, 1001, 2, 1), (3, 1002, 2, 5) |
| `pedidos_log` | `id` PK, `pedido_id` FK para `pedidos.id`, `evento` texto | (1, 1001, `criado`) |

Todas as colunas de texto e de data seguem a tabela de tipos de `tipos` (`varchar(40)`, `timestamp` e assim por diante, em cada dialeto).

**Ciclo mútuo** (`ciclo`): `departamentos` e `funcionarios` se apontam. `funcionarios` também aponta para si.

| Tabela | Colunas | Linhas |
| --- | --- | --- |
| `departamentos` | `id` PK, `codigo` único, `gerente_id` (aceita `NULL`) FK para `funcionarios.id` | (1, `dep-ti`, 1) |
| `funcionarios` | `id` PK, `codigo` único, `departamento_id` (aceita `NULL`) FK para `departamentos.id`, `chefe_id` (aceita `NULL`) FK para `funcionarios.id` | (1, `fun-ana`, 1, NULL), (2, `fun-bia`, 1, 1), (3, `fun-caio`, 1, 2) |

Como as duas tabelas se apontam, uma delas precisa ser criada antes da outra. Cada motor resolve isso de um jeito (SQLite aceita a referência adiante; Postgres e MySQL exigem criar as tabelas e adicionar a chave depois, ou desligar a checagem). Quem monta a origem na fixture usa o jeito do motor (`ALTER TABLE ... ADD FOREIGN KEY` depois dos dois `CREATE TABLE`, ou a ordem que o SQLite permitir).

**Sem chave estrangeira declarada** (`nome`): `contas` (`id` PK, `codigo`): (1, `conta-1`), (2, `conta-2`). `lancamentos` (`id` PK, `conta_id` int, `valor` numeric(10,2)): (1, 1, 10.00), (2, 1, 20.00), (3, 2, 5.00). Nenhuma `FOREIGN KEY`.

**Chave primária de texto** (`pk_texto`): `paises` (`sigla` varchar(2) PK, `nome`): (`BR`, `Brasil`), (`PT`, `Portugal`). `cidades` (`id` PK, `codigo` único, `nome`, `pais_sigla` FK para `paises.sigla`): (5, `cid-rj`, `Rio`, `BR`), (7, `cid-lis`, `Lisboa`, `PT`).

**Nomes difíceis** (`identificadores`): uma tabela chamada `ta"b ela` (aspa dupla e espaço no nome) com as colunas `id` (PK), `group` (palavra reservada), `co"l` (aspa dupla no nome) e `co` + crase + `l` (crase no nome). Uma linha: (1, `g`, `a`, `b`). Em cada motor, crie com o escape nativo (`"` duplicada no Postgres e no SQLite, crase duplicada no MySQL).

## O que significa "igual"

O DDL de destino **não é igual** ao da origem, e isso é correto: `bigint` vira `BIGINT` ou `INTEGER`, `jsonb` vira `TEXT` no SQLite, `tinyint(1)` vira `BOOLEAN`. Por isso os testes comparam o **significado**, não o texto.

### Schema

Lê-se o catálogo dos dois bancos com `metadata.Discover` (a mesma função que o programa usa) e compara-se, para **cada tabela exportada**:

1. **Tabelas**: mesmo conjunto de nomes (sem diferenciar maiúsculas). Tabelas temporárias de mapeamento do `--new-ids` não devem aparecer no destino (elas vivem só na conexão de restauração).
2. **Colunas**: mesmos nomes, na mesma ordem.
3. **Aceita nulo**: igual, com uma exceção: a coluna da chave primária é `NOT NULL` nos dois lados.
4. **Chave primária**: mesmas colunas, na mesma ordem.
5. **Chaves estrangeiras**: o destino tem exatamente as chaves da origem cuja tabela pai também foi exportada. Chave cujo pai ficou de fora (por `--depth` ou `--ignore-table-suffix`) **não** aparece no destino. Com `--relations-by-name`, o destino pode ter chaves a mais (as inferidas).
6. **Família do tipo**: a família da coluna de destino precisa ser a mesma da origem, ou uma das equivalências abaixo. A família sai do nome do tipo em minúsculas, sem parênteses e sem `unsigned`.

| Família na origem | Famílias aceitas no destino |
| --- | --- |
| int | int |
| decimal | decimal |
| float | float |
| bool | bool; no MySQL, `tinyint`; no SQLite, qualquer tipo de afinidade numérica (`BOOLEAN` é o esperado) |
| texto | texto |
| binário | binário |
| json | json; no SQLite, texto |
| data | data |
| hora | hora |
| timestamp | timestamp (`timestamp`, `datetime`) |
| texto de tipo desconhecido (`MY_TYPE`, sem tipo) | texto |

Famílias, pelo nome normalizado: int (`int`, `integer`, `smallint`, `bigint`, `mediumint`, `tinyint`, `serial`...), decimal (`numeric`, `decimal`), float (`real`, `float`, `double`...), bool (`bool`, `boolean`, `tinyint(1)`), texto (`char`, `varchar`, `text`, `longtext`, `enum`, `uuid`, `interval`...), binário (`bytea`, `blob`, `binary`...), json, data (`date`), hora (`time`), timestamp (`timestamp`, `datetime`).

### Dados

Cada linha da origem é achada no destino pela chave natural `codigo` (nunca por `id`, para servir ao `--new-ids`). A comparação é por coluna, na forma normalizada:

| Família | Como se compara |
| --- | --- |
| int | mesmo número inteiro (texto sem zeros à esquerda) |
| decimal | mesmo valor numérico (`99.90` e `99.9` são iguais). Compare com `math/big.Rat`. |
| float | mesmo `float64`. A fixture só usa valores exatos em binário (1.5, -0.25, 0, 1e10). |
| bool | os dois são "verdadeiro" ou os dois "falso". Valem `true`, `TRUE`, `t`, `1` e `false`, `FALSE`, `f`, `0`. |
| texto | **bytes idênticos**. Sem aparar espaços, sem normalizar Unicode. Exceção: `character(n)` do Postgres devolve com espaços de preenchimento, então apara-se à direita só nessa coluna (`c_bpchar`). |
| binário | bytes idênticos, inclusive vazio, `0x00`, `0x27` e `0x5C`. Leia a coluna como `[]byte`. |
| json | os dois textos, depois de `json.Unmarshal`, são iguais com `reflect.DeepEqual`. Ordem de chaves e espaços não importam (o `jsonb` e o `json` do MySQL reescrevem o texto). O JSON `null` (texto) continua diferente do `NULL` do banco. |
| data | texto exato `AAAA-MM-DD`. Se o motor devolver com hora (`2024-05-01T00:00:00Z`), corta-se na 10ª posição **antes** de comparar; mas o SQLite de destino guarda texto, então ali o valor lido já tem que ser exatamente `AAAA-MM-DD`. |
| hora | extrai-se `HH:MM:SS[.fração]` do texto lido e compara-se isso. |
| timestamp | instante em UTC, `AAAA-MM-DD HH:MM:SS[.fração]`, sem fuso. Aceita-se `T` no lugar do espaço e `Z` no final. A fração precisa ser igual até microssegundos. |
| `NULL` | os dois são `NULL`. Nulo nunca é igual a texto vazio nem a `0`. |

Também se confere a **contagem de linhas** por tabela. Linhas a mais ou a menos falham o teste.

### Fuso horário

- O Postgres de origem usa `timezone=UTC` no DSN, e o teste não depende do fuso da máquina nem do container.
- `timestamptz` do Postgres chega em UTC (`...Z`). O destino guarda `timestamp` sem fuso, em UTC. O que se compara é o relógio em UTC (`2024-05-01 08:00:00` para o valor `10:00:00+02`).
- A fixture do SQLite só usa `Z`, nunca deslocamento, nos cenários de comparação. Um valor com deslocamento (`+02:00`) tem um teste só para ele (`FusoDeslocamentoNaoQuebra`), que confere apenas que a restauração não falha.

## Cenários e casos de teste

Convenções desta seção:

- **Matriz 9** = roda nas 9 combinações. **Diagonal** = só `pg_para_pg`, `mysql_para_mysql`, `sqlite_para_sqlite`.
- "Exportar" significa `export.Run(ctx, db, cfg)` com `cfg.Driver = <origem>`, `cfg.OutputDriver = <destino>`, `cfg.DSN = "unused"`. O SQLite de origem é aberto em memória com `cache=shared` e nome único por teste.
- "Restaurar" significa executar o texto retornado num banco destino vazio, com chaves estrangeiras **ligadas** (no SQLite, `?_pragma=foreign_keys(1)` no DSN).
- "Compara schema e dados" significa as regras de [O que significa "igual"](#o-que-significa-igual).
- O nome do teste Go é `Test<Nome>` e cada combinação é um `t.Run("<origem>_para_<destino>", ...)`.

### 1. Tipos e valores (fixture `tipos`)

| Nome | Roda em | Exportar | Esperado ao restaurar |
| --- | --- | --- | --- |
| `TiposRoundTrip` | Matriz 9 | `Table=tipos`, `Column=lote`, `Record=A` | Restaura sem erro. Compara schema e dados, com as 10 linhas. |
| `TiposNovosIDs` | Matriz 9 | igual, com `NewIDs=true` | Restaura sem erro. Compara schema e dados por `codigo`. O conjunto de `id` do destino **não** tem nenhum valor de 101 a 110 (os ids são gerados). A tabela temporária de mapa não existe depois. |
| `TiposLinhaUnica` | Diagonal | `Column=codigo`, `Record=r03-barras-aspas` | Só 1 linha no destino, igual à origem. Mostra que o filtro por coluna diferente de `id` funciona. |
| `TiposCadaLinhaIsolada` | Matriz 9 | um `Run` por linha (`Column=id`, `Record=101` a `110`), restaurando cada um num banco novo | Cada uma das 10 linhas restaura sozinha e fica igual à origem. Serve para achar qual linha quebra. |
| `TimestampFracao` | Matriz 9 | linhas 101 e 102 (coluna `c_ts_frac`) | A fração até microssegundos é preservada: `10:00:00.123456` e `23:59:59.999999` voltam iguais. Veja [Bugs prováveis](#bugs-prováveis-e-o-que-fazer). |

### 2. Tipos que só existem em um motor

| Nome | Roda em | Exportar | Esperado ao restaurar |
| --- | --- | --- | --- |
| `ExtrasPG` | origem postgres, 3 destinos | `Table=extras_pg`, `Column=lote`, `Record=A` | Restaura sem erro. Todas as colunas voltam com o mesmo texto: `1 day 02:03:04`, o UUID, `{1,2,3}`, `(1,2)`, `3.14159` (sem perder casas), `ab` (após aparar à direita), e `c_tstz` como `2024-05-01 08:00:00` (UTC). |
| `ExtrasMySQL` | origem mysql, 3 destinos | `Table=extras_mysql`, `Column=lote`, `Record=A` | Restaura sem erro. `c_flag` volta como bool verdadeiro/falso (no Postgres, `BOOLEAN`), `c_ano` 2024, `c_enum` texto `azul`. |
| `SemSinal` | origem mysql, 3 destinos | `Table=sem_sinal`, `Column=lote`, `Record=A` | Restaura sem erro e os dois números voltam iguais (4294967295 e 18446744073709551615). |
| `ExtrasSQLite` | origem sqlite, 3 destinos | `Table=extras_sqlite`, `Column=lote`, `Record=A` | Restaura sem erro. `abc` volta `abc`; 42 volta como texto `42`; `x` volta `x`; `c_bool1` bool verdadeiro; `c_big_sem_sinal` volta 7; `c_datahora` volta o relógio `2024-05-01 10:00:00`. |

### 3. Texto difícil: NUL

NUL (`0x00`) só é aceito em texto pelo MySQL.

| Nome | Origem | Destino | Preparo | Esperado |
| --- | --- | --- | --- | --- |
| `NulMySQLParaMySQL` | mysql | mysql | tabela `nul_teste (id bigint PK, codigo, lote, c_texto longtext)` com uma linha `c_texto = 'a' + NUL + 'b'` | O arquivo contém `a\0b` (barra e zero, dois caracteres). Restaura sem erro. O valor volta com 3 bytes idênticos aos da origem. |
| `NulMySQLParaPG` | mysql | postgres | a mesma tabela | O arquivo gerado contém o byte `0x00` cru. Restaurar **falha** (o Postgres recusa NUL em texto). O teste confere o erro, e não que o erro foi mascarado. |
| `NulMySQLParaSQLite` | mysql | sqlite | a mesma tabela | Restaurar **falha** (o `modernc.org/sqlite` corta o SQL no NUL). O teste confere o erro. |

Estes dois últimos fixam o que a [doc de valores](EXPORTACAO.md) já diz: o TraceSQL escreve o byte como veio e deixa o erro aparecer.

### 4. Grafos

| Nome | Roda em | Exportar | Esperado ao restaurar |
| --- | --- | --- | --- |
| `GrafoSimples` | Matriz 9 | grafo da loja, `Table=pedidos`, `Column=id`, `Record=1001`, sem `Depth` | Restaura sem erro. Tabelas no destino: `categorias`, `clientes`, `itens`, `pedidos`, `pedidos_log`, `produtos`. Contagens: `pedidos` 2, `clientes` 2, `itens` 3, `produtos` 2, `categorias` 3, `pedidos_log` 1 (13 linhas). Schema e dados iguais. As 6 chaves estrangeiras (`categorias.pai_id`, `produtos.categoria_id`, `pedidos.cliente_id`, `itens.pedido_id`, `itens.produto_id`, `pedidos_log.pedido_id`) existem no destino. |
| `GrafoNovosIDs` | Matriz 9 | igual, com `NewIDs=true` | Restaura sem erro. Cada tabela com PK inteira de uma coluna recebe ids gerados (os ids originais 9, 10, 100, 1001 não aparecem como conjunto). **As referências continuam certas**: para cada linha filha, o `codigo` do pai alcançado pela FK no destino é o mesmo `codigo` do pai na origem. Nenhuma chave estrangeira fica `NULL` onde não era `NULL` na origem. Contagens iguais às de `GrafoSimples`. |
| `GrafoPKTexto` | Matriz 9 | `Table=cidades`, `Column=id`, `Record=5`, depois repetir com `NewIDs=true` | Sem `NewIDs`: `cidades` 1, `paises` 1 (`BR`). Com `NewIDs`: `paises` mantém `sigla = BR` (PK não inteira não é regenerada), `cidades.id` é gerado, e `cidades.pais_sigla` continua `BR`. |
| `CicloAuto` | Matriz 9 | `Table=categorias`, `Column=id`, `Record=100` (a mais nova, filha da filha) | Restaura sem erro. 3 linhas. `categorias.pai_id` correto: 100 aponta para 10, 10 para 9, 9 é `NULL`. A ordem de inserção respeita pai antes de filho **mesmo quando os ids ordenados como texto invertem a ordem** (`"10"` vem antes de `"9"`). |
| `CicloAutoNovosIDs` | Matriz 9 | igual, com `NewIDs=true` | Restaura sem erro. 3 linhas, a árvore (raiz, filha, neta) preservada por `codigo`. |
| `CicloMutuo` | Matriz 9 | `ciclo`, `Table=funcionarios`, `Column=id`, `Record=3` | O grafo reúne os dois lados (3 funcionários e 1 departamento). Restaura sem erro **e** com todas as referências: `departamentos.gerente_id = 1`, `funcionarios.departamento_id = 1` nas 3 linhas, `chefe_id` = NULL, 1, 2. O DDL existe com as duas chaves (cada lado aponta para o outro). |
| `CicloMutuoNovosIDs` | Matriz 9 | igual, com `NewIDs=true` | Restaura sem erro. Todas as referências por `codigo` iguais às da origem. Nenhuma referência vira `NULL` por ter sido escrita antes do pai existir. |

### 5. Opções

Todas em **diagonal**, no grafo da loja (`Table=pedidos`, `Column=id`, `Record=1001`). A tabela de contagens por profundidade foi calculada seguindo pais e filhos a partir do pedido 1001:

| `Depth` | O que entra além do que já tinha | Linhas no total | Tabelas |
| --- | --- | --- | --- |
| `0` | só o pedido 1001 | 1 | `pedidos` |
| `1` | cliente 10; itens 1 e 2; log 1 | 5 | `pedidos`, `clientes`, `itens`, `pedidos_log` |
| `2` | produtos 1 e 2 | 7 | + `produtos` |
| `3` | categorias 100 e 10; item 3 | 10 | + `categorias` |
| `4` | categoria 9; pedido 1002 | 12 | |
| `5` | cliente 20 | 13 | |
| `6` ou sem limite | nada novo | 13 | |

| Nome | Roda em | Exportar | Esperado ao restaurar |
| --- | --- | --- | --- |
| `Depth0` | Diagonal | `Depth=0` | 1 linha em `pedidos`; o arquivo só tem `CREATE TABLE` de `pedidos`. A FK `pedidos.cliente_id` **não** aparece (pai fora do arquivo). Restaura sem erro. |
| `Depth1` | Diagonal | `Depth=1` | 5 linhas (1, 1, 2, 1). `itens.produto_id` e `itens.pedido_id` existem como valores, mas a FK para `produtos` **não** é criada (pai fora). Restaura sem erro, mesmo com `itens.produto_id` apontando para linhas que não vieram. |
| `Depth2` a `Depth5` | Diagonal | `Depth=2..5` | Total de linhas da tabela acima (7, 10, 12, 13). Restaura sem erro. |
| `DepthSemLimite` | Diagonal | `Depth=nil` | 13 linhas. Igual a `GrafoSimples`. |
| `IgnoraSufixo` | Diagonal | `IgnoreTableSuffix=_log` | 12 linhas, sem `pedidos_log` no destino (nem o `CREATE TABLE`). Restaura sem erro. |
| `IgnoraSufixoTabelaInicial` | Diagonal | `Table=pedidos_log`, `Column=id`, `Record=1`, `IgnoreTableSuffix=_log` | A tabela inicial é exportada mesmo combinando com o sufixo (1 linha em `pedidos_log` e as ligadas por pais). Restaura sem erro. |
| `RelacoesPorNomeLigado` | Diagonal | `nome`, `Table=contas`, `Column=id`, `Record=1`, `RelationsByName=true` | 1 conta e 2 lançamentos (1 e 2; o 3 é da conta 2). O destino tem a FK `lancamentos.conta_id -> contas.id` (inferida). Restaura sem erro. |
| `RelacoesPorNomeDesligado` | Diagonal | igual, com `RelationsByName=false` | Só 1 linha (a conta 1). Sem `lancamentos` no destino. |

### 6. Nomes difíceis

| Nome | Roda em | Exportar | Esperado ao restaurar |
| --- | --- | --- | --- |
| `IdentificadoresDificeis` | Matriz 9 | `Table=ta"b ela` (aspa dupla e espaço), `Column=id`, `Record=1` | Restaura sem erro. A tabela e as 4 colunas (`id`, `group`, `co"l`, `co`+crase+`l`) existem com os nomes **exatos**. A linha volta igual. Veja [IDENTIFICADORES](IDENTIFICADORES.md). |

### 7. Fuso

| Nome | Roda em | Preparo | Esperado |
| --- | --- | --- | --- |
| `FusoDeslocamentoNaoQuebra` | origem sqlite, 3 destinos | coluna `TIMESTAMP` com o texto `2024-05-01T10:00:00+02:00` | Restaurar não dá erro e a linha existe. O valor exato não é conferido (veja a limitação em [Bugs prováveis](#bugs-prováveis-e-o-que-fazer)). |

## Rodando localmente

### Só SQLite

```bash
go test ./tests/integration/... -count=1
```

Os testes do Postgres e do MySQL aparecem como `SKIP` com o nome da variável que falta.

### Com Postgres e MySQL em containers

Tudo em PowerShell ou bash. Sem Go na máquina, use o container do Go na mesma rede dos bancos.

```bash
docker network create tracesql-it
docker run --rm -d --name tracesql-it-pg    --network tracesql-it -e POSTGRES_PASSWORD=it   postgres:16
docker run --rm -d --name tracesql-it-mysql --network tracesql-it -e MYSQL_ROOT_PASSWORD=it mysql:8

# Opcional (só com TRACESQL_IT_MARIADB_DSN):
docker run --rm -d --name tracesql-it-maria --network tracesql-it -e MARIADB_ROOT_PASSWORD=it mariadb:11
```

Espere ficar pronto. O MySQL leva alguns segundos e reinicia uma vez na primeira subida.

```bash
docker exec tracesql-it-pg pg_isready -U postgres
docker exec tracesql-it-mysql mysqladmin ping -uroot -pit --silent
```

Rode os testes (mesma rede, DSN com o nome do container):

```bash
docker run --rm --network tracesql-it \
  -v "$PWD":/src -w /src \
  -e GOTOOLCHAIN=auto \
  -e TRACESQL_IT_REQUIRE=1 \
  -e 'TRACESQL_IT_POSTGRES_DSN=postgres://postgres:it@tracesql-it-pg:5432/postgres?sslmode=disable&timezone=UTC' \
  -e 'TRACESQL_IT_MYSQL_DSN=root:it@tcp(tracesql-it-mysql:3306)/mysql?charset=utf8mb4&multiStatements=true' \
  golang:1.26.0 go test ./tests/integration/... -count=1 -v
```

Se o Go está instalado na máquina, publique as portas (`-p 55432:5432`, `-p 53306:3306`) e use `127.0.0.1` nos DSN, como no [começo rápido](#começo-rápido).

Para limpar:

```bash
docker rm -f tracesql-it-pg tracesql-it-mysql tracesql-it-maria
docker network rm tracesql-it
```

### Variáveis de ambiente

| Variável | Para que serve |
| --- | --- |
| `TRACESQL_IT_POSTGRES_DSN` | DSN de administração do Postgres (usuário que pode `CREATE DATABASE`). Precisa de `timezone=UTC`. |
| `TRACESQL_IT_MYSQL_DSN` | DSN de administração do MySQL (por exemplo `root`). O código acrescenta `multiStatements=true` e `charset=utf8mb4` se faltarem, e **não** usa `parseTime=true`. |
| `TRACESQL_IT_MARIADB_DSN` | Igual ao do MySQL, para o MariaDB. Opcional. |
| `TRACESQL_IT_REQUIRE` | Com `1`, DSN ausente de Postgres ou MySQL falha em vez de pular. |

Os nomes começam com `TRACESQL_IT_` para não colidir com as variáveis do programa (`TRACESQL_DSN`, `TRACESQL_DRIVER`...), que o teste nunca lê.

Cada teste cria bancos próprios com nome único (`it_<hash do teste>_src` e `_dst`) a partir do DSN de administração, e os apaga ao final. Rodar de novo, ou em paralelo, não deixa sujeira nem conflita. O DSN dos bancos criados é o DSN de administração com o nome do banco trocado.

## No CI

Um job novo, separado, em `.github/workflows/ci.yml`:

- Nome: `integration`. Roda em `ubuntu-latest`, em todo push e pull request, ao lado do job `test`.
- **É obrigatório.** Entra no `needs` do job `notify`, como o `test`. Motivo: o job usa service containers, é rápido (poucos minutos) e é a única proteção contra SQL que o banco não aceita.
- Serviços:
  - `postgres:16` com `POSTGRES_PASSWORD`, porta 5432 e `--health-cmd pg_isready`.
  - `mysql:8` com `MYSQL_ROOT_PASSWORD`, porta 3306 e `--health-cmd "mysqladmin ping -h 127.0.0.1 -proot"`.
  - Ambos com `health-interval`, `health-timeout` e `health-retries` para esperar a subida.
- Passos: checkout e setup do Go com as mesmas actions já fixadas por SHA no `ci.yml`; depois `go test ./tests/integration/... -count=1 -v`, com as variáveis `TRACESQL_IT_*` apontando para `127.0.0.1` e `TRACESQL_IT_REQUIRE=1`.
- O MariaDB **não** roda no `integration`. Ele entra num job separado, só no agendamento semanal (`schedule`) que já existe no workflow, com `mariadb:11`.
- O job `test` (Windows, macOS, Linux) continua com `go test ./...`. Ele roda os testes do SQLite desta suíte e pula os de Postgres e MySQL. Assim os três sistemas operacionais cobrem o SQLite, e só o Linux precisa de Docker.
- O `vulncheck` não muda.

Casos conhecidos que falham por bug real não podem derrubar o job: veja [Bugs prováveis](#bugs-prováveis-e-o-que-fazer).

## Decisões

Pontos que a issue não definia. Os padrões escolhidos são os mais seguros e os que pedem menos dependências.

1. **Sem dependência nova.** Nada de `testcontainers-go`. A suíte só usa os drivers que o programa já tem (`pgx`, `go-sql-driver/mysql`, `modernc.org/sqlite`), a biblioteca padrão e `docker run` (local) ou service containers (CI). Motivo: `testcontainers` traz dezenas de pacotes indiretos e exige Docker em qualquer máquina.
2. **Sem build tag.** `go test ./...` continua rodando só o que não precisa de banco externo, porque Postgres e MySQL são pulados quando falta o DSN. Uma tag escondida faria o teste sumir do `go vet` e do editor. A variável `TRACESQL_IT_REQUIRE=1` impede que o CI pule em silêncio.
3. **Pacote próprio `tests/integration`** (pacote `integration`, ao lado de `tests/policy`). Ele importa `internal/export`, `internal/metadata`, `internal/config` e `internal/db`, que são acessíveis de dentro do módulo. Os testes só usam a API exportada: `export.Run`, `metadata.Discover`, `config.Config` e `db.Open`.
4. **O que se testa é `export.Run` mais restaurar o texto**, não o binário. O CLI (Cobra e prompts) já tem testes próprios e não entra aqui.
5. **SQLite sempre roda, em memória.** Cada teste usa um nome único (`file:<nome>?mode=memory&cache=shared`). O destino SQLite liga as chaves estrangeiras (`_pragma=foreign_keys(1)`), porque o padrão desliga e esconderia erros.
6. **Um banco novo por teste e por motor**, criado e apagado pelo teste. Sem estado compartilhado entre testes.
7. **Versões das imagens:** `postgres:16` e `mysql:8` no CI e na doc. `mariadb:11` opcional, só no agendamento. Os mesmos motores do levantamento de tipos da issue #62.
8. **A mesma conexão para restaurar.** O arquivo todo roda numa conexão só, por causa das tabelas temporárias do `--new-ids`.
9. **Igual quer dizer equivalente**, nunca texto igual no DDL. Tipos são comparados por família; dados, por valor normalizado, exceto texto e binário, que são idênticos byte a byte.
10. **A chave natural `codigo` liga origem e destino**, para o `--new-ids` não depender de `id`.
11. **Fixture nativa por motor.** O DDL da origem é escrito à mão em cada dialeto, porque o programa lê o tipo que o motor informa.
12. **O comportamento esperado de ciclos é "restaura inteiro e com as referências".** Isto vale mesmo que o código de hoje não consiga. Quando não conseguir, o teste revela um bug (veja abaixo). Não se muda a expectativa para caber no código.
13. **Postgres de origem com `timezone=UTC`**, para o resultado não depender do fuso do container.
14. **MySQL de origem sem `parseTime`**, que é o padrão do programa. Datas chegam como texto.
15. **Cenários de opções só na diagonal**; os de tipos, grafos, ciclos, `--new-ids` e nomes, nas 9 combinações.
16. **O job `integration` é obrigatório.** O MariaDB fica só no agendamento semanal.
17. **NUL e fuso com deslocamento** têm testes próprios, sem entrar na comparação geral, porque as limitações já estão documentadas em [EXPORTACAO](EXPORTACAO.md).

## Bugs prováveis e o que fazer

Lendo o código, estas situações têm boa chance de falhar na primeira rodada. Cada uma é uma lacuna real, não um erro da suíte. A lista é uma previsão: confirme com o teste.

| # | Onde provavelmente falha | Por quê |
| --- | --- | --- |
| 1 | `pg_para_*`, `ExtrasPG` | `interval` e `point` contêm `int` no nome. O mapeamento de tipos do `CREATE TABLE` usa busca por pedaço do nome (o mesmo erro que a regra de [valores](EXPORTACAO.md) já corrigiu), e a coluna sai `INTEGER`. O `INSERT '1 day'` falha no Postgres e no MySQL. |
| 2 | `mysql_para_pg`, `ExtrasMySQL` e `TiposRoundTrip` | `tinyint(1)` do MySQL chega como `TINYINT`, e o valor sai como `1` (número). No Postgres a coluna é `BOOLEAN`, e `1` não é aceito como booleano. |
| 3 | `SemSinal`, `mysql_para_pg` e `mysql_para_sqlite` | `int unsigned` e `bigint unsigned` viram `INTEGER` e `BIGINT`, que não cabem 4294967295 (no Postgres) nem 18446744073709551615. |
| 4 | `CicloAuto`, `*_para_pg` e `*_para_mysql` | Os ids são ordenados como **texto** dentro da tabela. `"10"` vem antes de `"9"`, então o filho é inserido antes do pai e a FK recusa. |
| 5 | `CicloMutuo`, `*_para_pg` e `*_para_mysql` | Duas tabelas que se apontam: uma é criada antes da outra, e a FK para a tabela que ainda não existe falha. Depois, mesmo criadas, os `INSERT`s se bloqueiam (cada linha precisa da outra primeiro). |
| 6 | `CicloMutuoNovosIDs` | A referência para uma linha ainda não inserida vira `(SELECT new_id ... WHERE old_id = x)`, que devolve `NULL`. A referência se perde **sem erro**. Este é o mais perigoso: teste a ausência de `NULL` indevido. |
| 7 | `ExtrasPG`, `*_para_mysql` | `numeric` sem precisão vira `DECIMAL`, que no MySQL é `DECIMAL(10,0)`. `3.14159` vira `3`. |
| 8 | `TimestampFracao`, `*_para_mysql` | `timestamp(6)` e `datetime(6)` viram `DATETIME`, que arredonda frações. `23:59:59.999999` pode virar `2000-01-01 00:00:00`. |
| 9 | `pg_para_sqlite` e `pg_para_mysql`, coluna `date` | O Postgres entrega `2024-05-01T00:00:00Z`. O destino SQLite guarda texto com hora numa coluna `DATE`, e o teste exige `AAAA-MM-DD`. No MySQL, a hora numa coluna `DATE` pode virar aviso ou erro conforme o modo SQL. |
| 10 | `FusoDeslocamentoNaoQuebra` e `ExtrasPG`, `c_tstz` | `timestamptz` vira `TIMESTAMP` sem fuso. Um deslocamento que sobrar no texto (`+02:00`) é ignorado pelo Postgres, e o relógio muda. Aceita-se como limitação conhecida; o teste só trava falha de restauração. |
| 11 | `*_para_pg` com `--new-ids` | Depois de restaurar com os ids originais, a sequência da identidade do Postgres não avança. É um aviso para uso real (inserir sem `id` colide). Não há teste; registre se aparecer. |

### O que fazer se um teste revelar um bug

1. **Não corrija no mesmo PR**, salvo correção trivial: uma troca de poucas linhas, sem decisão de regra de negócio e que a doc já descreve. Qualquer outra coisa vira issue.
2. **Abra uma issue nova** por bug, com o rótulo `bug`. Inclua: a combinação (`origem_para_destino`), o nome do teste, o trecho de SQL que falhou, a mensagem de erro do banco, o esperado e o obtido. Se a correção mudar uma regra, a persona de documentação atualiza a doc antes do código.
3. **Mantenha o teste, mas marque-o**: `t.Skip("bug #<número>: <resumo em uma linha>")`, dentro do subteste da combinação afetada. O resto da matriz continua rodando. Assim o job `integration` fica verde, e o `grep "bug #"` lista o que está pendente.
4. **Quando a correção entrar**, o PR dela remove o `t.Skip` do teste correspondente. É o teste que prova a correção.
5. **Nunca mude a expectativa para caber no código.** A expectativa vem desta doc. Se a doc estiver errada, corrija a doc primeiro.
6. Um bug **de segurança** (por exemplo, um valor que fecha o literal) segue o fluxo de [SECURITY](SECURITY.md), não uma issue pública.

## Limitações da suíte

- Só cobre `export.Run` e a restauração do `.sql`. Não mede desempenho nem volume grande.
- Só um tipo por família na fixture comum; tipos raros (`money`, `bit`, `geometry`, `xml`) ficam fora, e suas limitações já estão em [EXPORTACAO](EXPORTACAO.md).
- Sem testes de conjuntos de caracteres que não sejam UTF-8.
- O MySQL é testado com o modo SQL padrão do `mysql:8` (estrito, sem `NO_BACKSLASH_ESCAPES`). O Postgres, com `standard_conforming_strings = on`. Mudar esses modos está fora do escopo.
- Chaves compostas e relações por nome além do caso básico ficam nos testes unitários ([RELACOES](RELACOES.md)).
