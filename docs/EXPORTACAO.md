# Como os valores viram literais SQL

Este documento explica como o TraceSQL escreve cada valor nos `INSERT`s do arquivo `.sql`.

Veja também: [README](../README.md), [ROADMAP](../ROADMAP.md) e [SEGURANCA](SEGURANCA.md).

## Por que isso importa

O TraceSQL lê valores do banco de origem como texto. Para escrever o `INSERT`, ele precisa decidir se o valor vira número cru (`42`), booleano (`TRUE`), binário (`X'4869'`) ou texto entre aspas (`'abc'`).

A decisão usa duas informações:

1. O **tipo da coluna na origem**, como o driver informa (`INT4`, `BYTEA`, `TINYINT(1)`, `varchar(20)`).
2. O **dialeto de saída** (`postgres`, `mysql` ou `sqlite`), escolhido por `--output-driver`.

A regra é: cada tipo cai em **uma categoria**, e cada categoria tem **um literal por dialeto**. A categoria vem de uma lista fechada de nomes de tipo. Nunca se procura um pedaço do nome dentro do nome.

### O erro que esta regra corrige

Antes, o código procurava pedaços como `int` dentro do nome do tipo. Por isso `interval` e `point` (que contêm `int`) saíam como número cru, e o `INSERT` ficava inválido:

| Tipo | Valor | Saída incorreta (antes) | Saída correta |
| --- | --- | --- | --- |
| `interval` | `1 day` | `1 day` | `'1 day'` |
| `point` | `(1,2)` | `(1,2)` | `'(1,2)'` |
| `bytea` | bytes `Hi` | `'Hi'` (texto) | `'\x4869'` no postgres, `X'4869'` no mysql e no sqlite |
| `jsonb` | `{"a":1}` | `'{"a":1}'` | `'{"a":1}'` (igual, agora por categoria explícita) |

### O segundo erro: a barra invertida no MySQL

Dentro de um texto entre aspas, o MySQL (no modo padrão) trata `\` como início de escape. Antes, o TraceSQL só dobrava a aspa simples e deixava a barra passar. O valor mudava ou o `INSERT` quebrava, e um texto malicioso podia fechar a aspa e injetar comandos ao restaurar o arquivo.

| Valor lido | Saída incorreta (antes, mysql) | O que o MySQL entende | Saída correta (mysql) |
| --- | --- | --- | --- |
| `C:\temp` | `'C:\temp'` | `C:` + tabulação + `emp` (o `\t` virou tabulação) | `'C:\\temp'` |
| `C:\temp\'x` | `'C:\temp\''x'` | a aspa dobrada é lida como escapada e o texto continua aberto | `'C:\\temp\\''x'` |

Por isso o escape do conteúdo depende do **dialeto de saída**. No postgres e no sqlite a barra invertida é um caractere comum e não deve ser alterada.

Outra regra de segurança: **valor numérico só sai cru se o texto for de fato um número**. Se não for, sai entre aspas. Assim, um texto inesperado nunca entra no SQL sem aspas.

## Exemplo

Uma linha de `orders` no postgres, exportada para mysql:

| Coluna | Tipo na origem | Valor lido | Literal escrito |
| --- | --- | --- | --- |
| `id` | `INT8` | `10` | `10` |
| `total` | `NUMERIC` | `99.90` | `99.90` |
| `paid` | `BOOL` | `t` | `TRUE` |
| `note` | `TEXT` | `it's ok` | `'it''s ok'` |
| `path` | `TEXT` | `C:\temp` | `'C:\\temp'` (no postgres e no sqlite seria `'C:\temp'`) |
| `meta` | `JSONB` | `{"a":1}` | `'{"a":1}'` |
| `created_at` | `TIMESTAMPTZ` | `2024-05-01T10:00:00Z` | `'2024-05-01 10:00:00'` |
| `deleted_at` | `TIMESTAMPTZ` | (nulo) | `NULL` |

## Referência

### Passo 1: normalizar o nome do tipo

Antes de classificar, o nome do tipo passa por estes passos, nesta ordem:

1. Remover espaços no início e no fim, e passar para minúsculas.
2. Se o nome inteiro, aparado e em minúsculas, é exatamente `tinyint(1)`, ele é mantido como está e o passo 3 não se aplica. Em qualquer outro nome, remover os trechos entre parênteses, com o parêntese.
3. (Só para os demais nomes.) Remover as palavras `unsigned`, `signed` e `zerofill`, em qualquer posição.
4. Remover o sufixo `with time zone` e o sufixo `without time zone`.
5. Trocar sequências de espaços por um só espaço e remover espaços nas pontas.

Exemplos:

| Nome recebido | Nome normalizado |
| --- | --- |
| `NUMERIC(10,2)` | `numeric` |
| `VARCHAR(20)` | `varchar` |
| `INT UNSIGNED` | `int` |
| `UNSIGNED BIGINT` | `bigint` |
| `Double  Precision` | `double precision` |
| `timestamp(6) with time zone` | `timestamp` |
| `TINYINT(1)` | `tinyint(1)` (mantido) |
| `TINYINT(1) UNSIGNED` | `tinyint` (não é a exceção) |
| `  Text ` | `text` |

### Passo 2: achar a categoria

Compara-se o nome normalizado **inteiro** com a lista da categoria. Não se aceita pedaço do nome.

| Categoria | Nomes normalizados |
| --- | --- |
| `bool` | `bool`, `boolean`, `tinyint(1)` |
| `numeric` | `int`, `integer`, `int1`, `int2`, `int3`, `int4`, `int8`, `tinyint`, `smallint`, `mediumint`, `bigint`, `serial`, `smallserial`, `bigserial`, `decimal`, `dec`, `numeric`, `fixed`, `float`, `float4`, `float8`, `real`, `double`, `double precision`, `year` |
| `binary` | `bytea`, `blob`, `tinyblob`, `mediumblob`, `longblob`, `binary`, `varbinary` |
| `json` | `json`, `jsonb` |
| `datetime` | `date`, `time`, `timetz`, `timestamp`, `timestamptz`, `datetime`, `datetime2`, `smalldatetime` |
| `text` | todo o resto (lista abaixo) |

Pontos de atenção:

- Nome vazio é `text`.
- `integer[3]` e `int[3]` também são arrays (terminam com `]`) e saem como `text`.
- Data com `T` sem segundos, ou fora do formato `AAAA-MM-DDTHH:MM:SS[.frac][Z|±HH:MM]`, não é ajustada: sai inalterada entre aspas.
- **Array** é `text`. Um nome que começa com `_` (como `_int4`, `_text`) ou termina com `[]` (como `integer[]`) é array.
- `interval`, `point`, `uuid`, `inet`, `cidr`, `macaddr`, `xml`, `money`, `bit`, `varbit`, `enum`, `set`, `char`, `varchar`, `character`, `character varying`, `bpchar`, `name`, `citext`, `text`, `tinytext`, `mediumtext`, `longtext`, `clob` e `geometry` são `text`.
- **Tipo desconhecido é `text`.** Exemplo: `my_custom_type` sai entre aspas, nunca como número.
- A comparação não diferencia maiúsculas de minúsculas: `BIGINT`, `bigint` e `BigInt` são iguais.
- No MySQL, o driver informa `TINYINT` sem o tamanho. Por isso uma coluna `tinyint(1)` do MySQL chega como `numeric`, e o valor sai como `0` ou `1`. Isso é equivalente. `tinyint(1)` só vira `bool` quando o nome do tipo traz o `(1)` (por exemplo, o tipo declarado no SQLite).

### Passo 3: escrever o literal

Regra geral: se o valor lido é nulo (`Valid` falso), o literal é `NULL`, em qualquer categoria e dialeto. Um texto vazio **não** é nulo: em `text` sai `''`.

| Categoria | postgres | mysql | sqlite |
| --- | --- | --- | --- |
| `numeric` | o texto cru | o texto cru | o texto cru |
| `bool` | `TRUE` / `FALSE` | `TRUE` / `FALSE` | `1` / `0` |
| `binary` | `'\x<hex>'` | `X'<hex>'` | `X'<hex>'` |
| `json` | texto entre aspas | texto entre aspas | texto entre aspas |
| `datetime` | texto entre aspas, sem mudar | texto entre aspas, com ajuste de data (abaixo) | texto entre aspas, com ajuste de data (abaixo) |
| `text` | texto entre aspas | texto entre aspas | texto entre aspas |

**Texto entre aspas:** vale para todo literal que sai entre aspas simples: `text`, `json`, `datetime`, tipo desconhecido, array, e o texto de alternativa de um numérico ou booleano inválido. Só o binário fica de fora (ele usa hexadecimal). O conteúdo é escapado conforme o dialeto de saída e o resultado fica entre aspas simples:

| Caractere no valor | postgres | mysql | sqlite |
| --- | --- | --- | --- |
| `'` (aspa simples) | `''` | `''` | `''` |
| `\` (barra invertida) | `\` (inalterada) | `\\` | `\` (inalterada) |
| NUL (byte `0x00`) | inalterado | `\0` | inalterado |
| LF (quebra de linha, `0x0A`) | inalterada | `\n` | inalterada |
| CR (retorno de carro, `0x0D`) | inalterado | `\r` | inalterado |
| Ctrl-Z (byte `0x1A`) | inalterado | `\Z` | inalterado |
| `"`, tabulação e Unicode | inalterados | inalterados | inalterados |

No mysql, `\0`, `\n`, `\r` e `\Z` são dois caracteres: a barra invertida e a letra (ou o dígito). Cada caractere do valor é tratado uma só vez: a barra criada por um escape não é escapada de novo. Assim, um `\n` escrito no valor (barra e `n`) vira `\\n` no mysql e continua `\n` nos outros dois.

Por que cada dialeto é assim:

- **mysql:** com `NO_BACKSLASH_ESCAPES` desligado (o padrão), a barra é escape. O conjunto é o de `mysql_real_escape_string`, menos a aspa dupla, que dentro de aspas simples não precisa de escape. NUL e quebras de linha também são escapados para o arquivo não ter byte NUL cru nem valores quebrando linha.
- **postgres:** com `standard_conforming_strings = on` (o padrão desde o PostgreSQL 9.1), a barra é um caractere comum dentro de `'...'`. Por isso **não se usa** a forma `E'...'`. NUL não existe em texto do postgres, então não há escape que o salve.
- **sqlite:** a barra é sempre um caractere comum. NUL e quebras de linha ficam dentro de `'...'` como estão. (Restaurar um NUL com o driver `modernc.org/sqlite` falha; veja as limitações.)

O escape tem de ser a inversa exata da leitura do banco de destino: ao restaurar o arquivo, o texto guardado é idêntico ao original (veja os testes de ida e volta abaixo).

**Numérico:** o texto só sai cru se casar com `^-?(\d+(\.\d+)?|\.\d+)([eE][+-]?\d+)?$`. Se não casar, sai como texto entre aspas. Exemplos: `NaN`, `Infinity`, `-Infinity` e texto vazio saem como `'NaN'`, `'Infinity'`, `'-Infinity'` e `''`.

**Booleano:** o valor lido é verdadeiro se for `t`, `true` ou `1`, e falso se for `f`, `false` ou `0`. A comparação não diferencia maiúsculas de minúsculas. Qualquer outro valor sai como texto entre aspas (`'maybe'`), para que o erro apareça ao restaurar em vez de virar `FALSE` sem aviso.

**Binário:** o valor lido são os bytes brutos. O `<hex>` é cada byte em dois dígitos hexadecimais minúsculos. Binário vazio sai `'\x'` no postgres e `X''` no mysql e no sqlite.

**JSON:** sai como texto entre aspas, sem conversão (`::jsonb` etc.). O banco de destino converte o texto para o tipo da coluna.

**Data e hora (mysql e sqlite):** se o valor tem o formato `AAAA-MM-DDTHH:MM:SS` com frações opcionais, o `T` vira um espaço. Se o valor termina em `Z`, o `Z` é removido (o valor já está em UTC). Um deslocamento como `+02:00` é mantido. Valores sem `T` (como `2024-05-01`) não mudam. No postgres, nada muda.

**Dialeto de saída:** o nome é normalizado como no resto do programa (`postgresql` e `pg` valem `postgres`; `sqlite3` vale `sqlite`). Um dialeto desconhecido ou vazio usa as mesmas regras do `sqlite`.

### Onde a regra vale

O mesmo literal é usado em todo lugar onde o programa escreve um valor lido do banco: colunas dos `INSERT`s e valores de referência quando `--new-ids` está ativo (o `(SELECT new_id ... WHERE old_id = <literal>)`).

## Decisões

Pontos que o código e a issue não definiam. Os padrões escolhidos são os mais seguros.

1. **Tipo desconhecido vira texto entre aspas.** Nunca número cru.
2. **Numérico só sai cru se o texto for um número.** Caso contrário, sai entre aspas.
3. **Valor booleano inválido sai entre aspas**, não como `FALSE`.
4. **SQLite usa `1` e `0` para booleano**, porque versões antigas do SQLite não aceitam `TRUE` e `FALSE`.
5. **JSON não leva conversão de tipo** (`::jsonb`). O texto entre aspas basta e funciona nos três dialetos.
6. **Array vira texto entre aspas**, no formato do postgres (`'{1,2,3}'`). Em mysql e sqlite não existe tipo array, e o texto é guardado como veio.
7. **`bit` e `varbit` são texto.** `interval`, `point` e tipos de geometria também.
8. **Datas só são ajustadas para mysql e sqlite**, e só para o formato com `T` e `Z`. Não há conversão de fuso.
9. **Binário usa hexadecimal minúsculo**, sem conversão de fuso nem de codificação.
10. **`tinyint(1)` só é booleano quando o nome inteiro, aparado e em minúsculas, é exatamente `tinyint(1)`.** Com qualquer outra palavra junto, como `tinyint(1) unsigned`, o tipo é numérico.
11. **A barra invertida é escapada só no mysql**, e só no mysql NUL, LF, CR e Ctrl-Z viram escape. Postgres e sqlite não ganham `E'...'` nem outro prefixo.
12. **Aspa dupla e tabulação nunca são escapadas.** Dentro de aspas simples, o mysql não precisa disso.
13. **O escape vale para todo literal entre aspas**, de qualquer categoria. Não vale para o binário.
14. **Dialeto desconhecido ou vazio usa o escape do sqlite** (só dobra `'`).

## Limitações conhecidas

- O escape de nomes de tabela e coluna (issue #64) é tratado à parte, em [IDENTIFICADORES.md](IDENTIFICADORES.md).
- No postgres, o arquivo supõe `standard_conforming_strings = on` (o padrão). Com o ajuste desligado no destino, a barra invertida em texto seria lida como escape. O TraceSQL não escreve `SET standard_conforming_strings` no arquivo.
- No mysql, o arquivo supõe `NO_BACKSLASH_ESCAPES` desligado (o padrão). Com ele ligado, `\\` seria lido como duas barras.
- NUL em texto não pode ser restaurado no postgres (o banco recusa). O TraceSQL escreve o byte como veio e deixa o erro aparecer.
- NUL em texto também não pode ser restaurado no sqlite com o driver `modernc.org/sqlite`: ele corta o SQL no byte NUL (`unrecognized token`), mesmo com o literal correto. O literal gerado continua com o NUL cru.
- O escape supõe texto em UTF-8. Conjuntos de caracteres em que um byte de caractere multibyte pode valer `0x5C` (como GBK e SJIS) não são tratados.
- Tipos espaciais do MySQL (`geometry`, `point`) chegam como bytes e saem como texto entre aspas. Eles não são restaurados corretamente.
- Os nomes exatos dos tipos dependem do driver de origem. A lista do Passo 2 cobre os nomes que `pgx`, `go-sql-driver/mysql` e `modernc.org/sqlite` informam.

## Como os testes devem conferir

Os testes ficam no pacote `export` (`internal/export`) e chamam duas funções, sem banco de dados:

- `classifyType(typeName string) typeCategory` devolve a categoria. As constantes são `categoryNumeric`, `categoryBool`, `categoryBinary`, `categoryJSON`, `categoryDatetime` e `categoryText`.
- `formatValue(outputDriver, typeName string, v sql.NullString) string` devolve o literal. Para um valor nulo, passe `sql.NullString{}`; para um valor, `sql.NullString{String: "...", Valid: true}`.

`formatValue` não recebe mais o `*sql.ColumnType`. Quem chama passa `colType.DatabaseTypeName()`.

Cada linha das tabelas abaixo é um caso. Use a tabela como `[]struct{name, ...}` e `t.Run(name, ...)`. A coluna "Valor" mostra o texto lido; `(nulo)` significa `sql.NullString{}`; `(vazio)` significa `Valid: true` e `String: ""`.

### Classificação (`classifyType`)

| Nome do caso | Tipo | Categoria |
| --- | --- | --- |
| `int_minusculo` | `int` | numeric |
| `integer_maiusculo` | `INTEGER` | numeric |
| `int4_pgx` | `INT4` | numeric |
| `bigint_misto` | `BigInt` | numeric |
| `int_unsigned_sufixo` | `INT UNSIGNED` | numeric |
| `unsigned_bigint_prefixo` | `UNSIGNED BIGINT` | numeric |
| `tinyint_sem_tamanho` | `TINYINT` | numeric |
| `numeric_com_parametros` | `numeric(10,2)` | numeric |
| `decimal_com_parametros` | `DECIMAL(10, 2)` | numeric |
| `double_precision` | `double precision` | numeric |
| `double_precision_espacos` | `Double  Precision` | numeric |
| `float8` | `FLOAT8` | numeric |
| `real` | `REAL` | numeric |
| `year_mysql` | `YEAR` | numeric |
| `serial` | `serial` | numeric |
| `bool_pgx` | `BOOL` | bool |
| `boolean` | `boolean` | bool |
| `tinyint_1_bool` | `tinyint(1)` | bool |
| `tinyint_1_maiusculo` | `TINYINT(1)` | bool |
| `tinyint_1_unsigned_maiusculo` | `TINYINT(1) UNSIGNED` | numeric |
| `tinyint_1_unsigned_minusculo` | `tinyint(1) unsigned` | numeric |
| `array_com_tamanho_integer` | `integer[3]` | text |
| `array_com_tamanho_int` | `int[3]` | text |
| `tinyint_4_numerico` | `tinyint(4)` | numeric |
| `bytea` | `BYTEA` | binary |
| `blob` | `BLOB` | binary |
| `longblob` | `LONGBLOB` | binary |
| `varbinary_com_tamanho` | `VARBINARY(16)` | binary |
| `binary_com_tamanho` | `binary(16)` | binary |
| `json` | `JSON` | json |
| `jsonb` | `JSONB` | json |
| `date` | `DATE` | datetime |
| `time_com_precisao` | `time(3)` | datetime |
| `timestamp` | `timestamp` | datetime |
| `timestamptz` | `TIMESTAMPTZ` | datetime |
| `timestamp_with_time_zone` | `timestamp(6) with time zone` | datetime |
| `datetime_mysql` | `DATETIME` | datetime |
| `interval_nao_e_numero` | `interval` | text |
| `point_nao_e_numero` | `point` | text |
| `uuid` | `UUID` | text |
| `varchar_com_tamanho` | `varchar(20)` | text |
| `character_varying` | `character varying` | text |
| `text` | `TEXT` | text |
| `money` | `money` | text |
| `bit` | `BIT` | text |
| `array_prefixo_underscore` | `_int4` | text |
| `array_prefixo_underscore_texto` | `_text` | text |
| `array_sufixo_colchetes` | `integer[]` | text |
| `desconhecido` | `my_custom_type` | text |
| `desconhecido_com_int` | `printer` | text |
| `vazio` | (vazio) | text |

O caso `desconhecido_com_int` garante que um nome que apenas contém `int` não vira número.

### Literais (`formatValue`)

A coluna "Saída" é o retorno exato, sem espaços extras.

**Nulo e vazio (todos os dialetos)**

| Nome do caso | Dialeto | Tipo | Valor | Saída |
| --- | --- | --- | --- | --- |
| `nulo_numerico` | postgres | `int4` | (nulo) | `NULL` |
| `nulo_texto` | mysql | `text` | (nulo) | `NULL` |
| `nulo_bool` | sqlite | `bool` | (nulo) | `NULL` |
| `nulo_binario` | postgres | `bytea` | (nulo) | `NULL` |
| `nulo_json` | mysql | `json` | (nulo) | `NULL` |
| `nulo_data` | sqlite | `timestamp` | (nulo) | `NULL` |
| `texto_vazio` | postgres | `text` | (vazio) | `''` |
| `numerico_vazio` | mysql | `int` | (vazio) | `''` |

**Numérico (igual nos três dialetos; repetir para postgres, mysql e sqlite)**

| Nome do caso | Tipo | Valor | Saída |
| --- | --- | --- | --- |
| `inteiro` | `int4` | `42` | `42` |
| `inteiro_negativo` | `bigint` | `-7` | `-7` |
| `unsigned_grande` | `UNSIGNED BIGINT` | `18446744073709551615` | `18446744073709551615` |
| `decimal` | `numeric(10,2)` | `99.90` | `99.90` |
| `decimal_sem_inteiro` | `decimal` | `.5` | `.5` |
| `double_cientifico` | `double precision` | `1.5e10` | `1.5e10` |
| `float_cientifico_negativo` | `float8` | `-2E-3` | `-2E-3` |
| `year` | `YEAR` | `2024` | `2024` |
| `nan` | `float8` | `NaN` | `'NaN'` |
| `infinito` | `float8` | `Infinity` | `'Infinity'` |
| `infinito_negativo` | `float8` | `-Infinity` | `'-Infinity'` |
| `texto_no_numerico` | `int` | `1; DROP TABLE x` | `'1; DROP TABLE x'` |
| `espaco_no_numerico` | `int` | ` 1` | `' 1'` |

**Não numéricos que continham "int" (igual nos três dialetos)**

| Nome do caso | Tipo | Valor | Saída |
| --- | --- | --- | --- |
| `interval` | `interval` | `1 day` | `'1 day'` |
| `interval_maiusculo` | `INTERVAL` | `2 years 3 mons` | `'2 years 3 mons'` |
| `point` | `point` | `(1,2)` | `'(1,2)'` |
| `desconhecido_com_int` | `printer` | `abc` | `'abc'` |

**Booleano**

| Nome do caso | Dialeto | Tipo | Valor | Saída |
| --- | --- | --- | --- | --- |
| `bool_t_postgres` | postgres | `bool` | `t` | `TRUE` |
| `bool_f_postgres` | postgres | `bool` | `f` | `FALSE` |
| `bool_true_mysql` | mysql | `boolean` | `true` | `TRUE` |
| `bool_false_mysql` | mysql | `boolean` | `false` | `FALSE` |
| `bool_1_mysql` | mysql | `tinyint(1)` | `1` | `TRUE` |
| `bool_0_mysql` | mysql | `tinyint(1)` | `0` | `FALSE` |
| `tinyint_1_unsigned_um` | mysql | `tinyint(1) unsigned` | `1` | `1` |
| `tinyint_1_unsigned_zero` | mysql | `TINYINT(1) UNSIGNED` | `0` | `0` |
| `bool_true_sqlite` | sqlite | `boolean` | `true` | `1` |
| `bool_false_sqlite` | sqlite | `boolean` | `false` | `0` |
| `bool_1_sqlite` | sqlite | `BOOL` | `1` | `1` |
| `bool_0_sqlite` | sqlite | `BOOL` | `0` | `0` |
| `bool_maiusculo` | postgres | `bool` | `TRUE` | `TRUE` |
| `bool_T_maiusculo` | sqlite | `bool` | `T` | `1` |
| `bool_invalido` | postgres | `bool` | `maybe` | `'maybe'` |
| `bool_vazio` | mysql | `bool` | (vazio) | `''` |
| `tinyint_numerico_mysql` | mysql | `TINYINT` | `1` | `1` |

**Binário**

O valor é uma `string` Go com os bytes brutos. `Hi` são os bytes `0x48 0x69`.

| Nome do caso | Dialeto | Tipo | Valor | Saída |
| --- | --- | --- | --- | --- |
| `bytea_postgres` | postgres | `BYTEA` | `Hi` | `'\x4869'` |
| `blob_mysql` | mysql | `BLOB` | `Hi` | `X'4869'` |
| `blob_sqlite` | sqlite | `BLOB` | `Hi` | `X'4869'` |
| `binario_com_zero_e_alto` | mysql | `longblob` | bytes `0x00 0xff 0x27` | `X'00ff27'` |
| `binario_com_zero_e_alto_pg` | postgres | `bytea` | bytes `0x00 0xff 0x27` | `'\x00ff27'` |
| `varbinary_sqlite` | sqlite | `varbinary(16)` | bytes `0x01 0x02` | `X'0102'` |
| `binario_vazio_postgres` | postgres | `bytea` | (vazio) | `'\x'` |
| `binario_vazio_mysql` | mysql | `blob` | (vazio) | `X''` |
| `binario_vazio_sqlite` | sqlite | `blob` | (vazio) | `X''` |

**JSON (igual nos três dialetos)**

| Nome do caso | Tipo | Valor | Saída |
| --- | --- | --- | --- |
| `json_objeto` | `jsonb` | `{"a":1}` | `'{"a":1}'` |
| `json_com_aspas_simples` | `json` | `{"a":"it's"}` | `'{"a":"it''s"}'` |
| `json_array` | `JSON` | `[1,2,3]` | `'[1,2,3]'` |
| `json_texto_nulo` | `jsonb` | `null` | `'null'` |

`json_texto_nulo` mostra que o texto `null` (JSON nulo) é diferente de `NULL` do banco: só `Valid: false` gera `NULL`.

**Array (igual nos três dialetos)**

| Nome do caso | Tipo | Valor | Saída |
| --- | --- | --- | --- |
| `array_inteiros` | `_int4` | `{1,2,3}` | `'{1,2,3}'` |
| `array_inteiros_colchetes` | `integer[]` | `{1,2,3}` | `'{1,2,3}'` |
| `array_texto_com_aspas` | `_text` | `{"it's"}` | `'{"it''s"}'` |

**Data e hora**

| Nome do caso | Dialeto | Tipo | Valor | Saída |
| --- | --- | --- | --- | --- |
| `data_postgres` | postgres | `date` | `2024-05-01` | `'2024-05-01'` |
| `timestamp_z_postgres` | postgres | `timestamptz` | `2024-05-01T10:00:00Z` | `'2024-05-01T10:00:00Z'` |
| `timestamp_z_mysql` | mysql | `TIMESTAMPTZ` | `2024-05-01T10:00:00Z` | `'2024-05-01 10:00:00'` |
| `timestamp_z_sqlite` | sqlite | `timestamp` | `2024-05-01T10:00:00Z` | `'2024-05-01 10:00:00'` |
| `timestamp_fracao_mysql` | mysql | `datetime` | `2024-05-01T10:00:00.123456Z` | `'2024-05-01 10:00:00.123456'` |
| `timestamp_deslocamento_mysql` | mysql | `timestamptz` | `2024-05-01T10:00:00+02:00` | `'2024-05-01 10:00:00+02:00'` |
| `timestamp_deslocamento_sqlite` | sqlite | `timestamptz` | `2024-05-01T10:00:00-03:00` | `'2024-05-01 10:00:00-03:00'` |
| `data_mysql` | mysql | `DATE` | `2024-05-01` | `'2024-05-01'` |
| `datetime_com_espaco_mysql` | mysql | `DATETIME` | `2024-05-01 10:00:00` | `'2024-05-01 10:00:00'` |
| `hora_sqlite` | sqlite | `time` | `10:00:00` | `'10:00:00'` |
| `data_t_sem_segundos_mysql` | mysql | `datetime` | `2024-05-01T10:00` | `'2024-05-01T10:00'` |
| `data_fora_do_formato_sqlite` | sqlite | `timestamp` | `2024-05-01T10:00:00 Z` | `'2024-05-01T10:00:00 Z'` |

**Texto e aspas**

| Nome do caso | Dialeto | Tipo | Valor | Saída |
| --- | --- | --- | --- | --- |
| `texto_simples` | postgres | `text` | `abc` | `'abc'` |
| `texto_aspas_simples` | mysql | `varchar(20)` | `it's` | `'it''s'` |
| `texto_varias_aspas` | sqlite | `TEXT` | `'a'b'` | `'''a''b'''` |
| `texto_numerico_continua_texto` | postgres | `varchar(20)` | `123` | `'123'` |
| `uuid` | postgres | `uuid` | `3f2b8c3e-0000-4000-8000-000000000000` | `'3f2b8c3e-0000-4000-8000-000000000000'` |
| `desconhecido` | mysql | `my_custom_type` | `42` | `'42'` |
| `tipo_vazio` | sqlite | (vazio) | `42` | `'42'` |

**Nome do dialeto**

| Nome do caso | Dialeto | Tipo | Valor | Saída |
| --- | --- | --- | --- | --- |
| `dialeto_pg` | `pg` | `bool` | `t` | `TRUE` |
| `dialeto_postgresql` | `postgresql` | `bytea` | `Hi` | `'\x4869'` |
| `dialeto_sqlite3` | `sqlite3` | `bool` | `true` | `1` |
| `dialeto_maiusculo` | `MySQL` | `blob` | `Hi` | `X'4869'` |
| `dialeto_desconhecido_usa_sqlite` | `oracle` | `bool` | `true` | `1` |
| `dialeto_vazio_usa_sqlite` | (vazio) | `bool` | `true` | `1` |

**Escape do conteúdo (issue #63)**

Notação desta seção, só para escrever os casos:

- O texto está em código e é o texto real, caractere por caractere.
- `<NUL>`, `<LF>`, `<CR>`, `<TAB>` e `<SUB>` (Ctrl-Z) são **um único byte** (`0x00`, `0x0A`, `0x0D`, `0x09`, `0x1A`). No teste, escreva com `"\x00"`, `"\n"`, `"\r"`, `"\t"` e `"\x1a"`.
- No mysql, `\n`, `\r`, `\0` e `\Z` na saída são **dois caracteres** (barra e letra ou dígito), não um byte de controle.
- Em todos os casos o tipo é `text`, salvo indicação. A coluna "Valor" é o valor lido (`Valid: true`).
- Nos casos em que as três saídas são iguais, a coluna é única.

Saídas por dialeto (use `t.Run` por caso e por dialeto):

| Nome do caso | Valor | postgres | mysql | sqlite |
| --- | --- | --- | --- | --- |
| `barra_simples` | `C:\temp` | `'C:\temp'` | `'C:\\temp'` | `'C:\temp'` |
| `barra_dupla` | `a\\b` | `'a\\b'` | `'a\\\\b'` | `'a\\b'` |
| `barra_no_fim` | `abc\` | `'abc\'` | `'abc\\'` | `'abc\'` |
| `so_barra` | `\` | `'\'` | `'\\'` | `'\'` |
| `barra_n_escrito` | `\n` (barra e `n`) | `'\n'` | `'\\n'` | `'\n'` |
| `barra_zero_escrito` | `\0` (barra e `0`) | `'\0'` | `'\\0'` | `'\0'` |
| `aspa_e_barra` | `it's \ ok` | `'it''s \ ok'` | `'it''s \\ ok'` | `'it''s \ ok'` |
| `barra_antes_de_aspa` | `C:\temp\'x` | `'C:\temp\''x'` | `'C:\\temp\\''x'` | `'C:\temp\''x'` |
| `injecao_aspa` | `'; DROP TABLE x;--` | `'''; DROP TABLE x;--'` | `'''; DROP TABLE x;--'` | `'''; DROP TABLE x;--'` |
| `injecao_com_barra` | `\'; DROP TABLE x;--` | `'\''; DROP TABLE x;--'` | `'\\''; DROP TABLE x;--'` | `'\''; DROP TABLE x;--'` |
| `nul` | `a<NUL>b` | `'a<NUL>b'` | `'a\0b'` | `'a<NUL>b'` |
| `quebra_lf` | `l1<LF>l2` | `'l1<LF>l2'` | `'l1\nl2'` | `'l1<LF>l2'` |
| `quebra_cr` | `l1<CR>l2` | `'l1<CR>l2'` | `'l1\rl2'` | `'l1<CR>l2'` |
| `quebra_crlf` | `l1<CR><LF>l2` | `'l1<CR><LF>l2'` | `'l1\r\nl2'` | `'l1<CR><LF>l2'` |
| `barra_antes_de_lf` | `a\<LF>b` | `'a\<LF>b'` | `'a\\\nb'` | `'a\<LF>b'` |
| `ctrl_z` | `a<SUB>b` | `'a<SUB>b'` | `'a\Zb'` | `'a<SUB>b'` |
| `tabulacao` | `a<TAB>b` | `'a<TAB>b'` | `'a<TAB>b'` | `'a<TAB>b'` |
| `aspas_duplas` | `say "hi"` | `'say "hi"'` | `'say "hi"'` | `'say "hi"'` |
| `unicode` | `ação 日本 😀` | `'ação 日本 😀'` | `'ação 日本 😀'` | `'ação 日本 😀'` |
| `unicode_com_barra` | `日\本` | `'日\本'` | `'日\\本'` | `'日\本'` |
| `vazio` | (vazio) | `''` | `''` | `''` |
| `sem_caractere_especial` | `abc` | `'abc'` | `'abc'` | `'abc'` |

No caso `barra_antes_de_lf` do mysql, a saída é: aspa, `a`, duas barras, barra e `n`, `b`, aspa (ou seja, `a`, `\\`, `\n`, `b`).

Outras categorias e outros caminhos que passam pelo mesmo escape:

| Nome do caso | Tipo | Valor | postgres | mysql | sqlite |
| --- | --- | --- | --- | --- | --- |
| `json_com_barra` | `jsonb` | `{"p":"C:\\temp"}` | `'{"p":"C:\\temp"}'` | `'{"p":"C:\\\\temp"}'` | `'{"p":"C:\\temp"}'` |
| `json_com_aspa_e_barra` | `json` | `{"a":"it's \\"}` | `'{"a":"it''s \\"}'` | `'{"a":"it''s \\\\"}'` | `'{"a":"it''s \\"}'` |
| `array_com_barra` | `_text` | `{"a\\b"}` | `'{"a\\b"}'` | `'{"a\\\\b"}'` | `'{"a\\b"}'` |
| `desconhecido_com_barra` | `my_custom_type` | `\x41` | `'\x41'` | `'\\x41'` | `'\x41'` |
| `data_fora_do_formato_com_barra` | `datetime` | `a\b` | `'a\b'` | `'a\\b'` | `'a\b'` |
| `data_valida_nao_muda` | `timestamp` | `2024-05-01T10:00:00Z` | `'2024-05-01T10:00:00Z'` | `'2024-05-01 10:00:00'` | `'2024-05-01 10:00:00'` |
| `numerico_invalido_com_barra` | `int` | `1\2` | `'1\2'` | `'1\\2'` | `'1\2'` |
| `bool_invalido_com_barra` | `bool` | `x\y` | `'x\y'` | `'x\\y'` | `'x\y'` |
| `binario_nao_e_escapado` | `bytea` (ou `blob` no mysql e no sqlite) | o byte `0x5C` | `'\x5c'` | `X'5c'` | `X'5c'` |
| `nulo_continua_null` | `text` | (nulo) | `NULL` | `NULL` | `NULL` |

Nos casos `json_*`, `array_*` e `data_*`, a barra dupla do valor lido são duas barras reais (JSON já escapado).

Dialetos (valor `a\b'c`, tipo `text`):

| Nome do caso | Dialeto | Saída |
| --- | --- | --- |
| `dialeto_mysql_maiusculo` | `MySQL` | `'a\\b''c'` |
| `dialeto_pg` | `pg` | `'a\b''c'` |
| `dialeto_postgresql` | `postgresql` | `'a\b''c'` |
| `dialeto_sqlite3` | `sqlite3` | `'a\b''c'` |
| `dialeto_desconhecido` | `oracle` | `'a\b''c'` |
| `dialeto_vazio` | (vazio) | `'a\b''c'` |

Referência com `--new-ids`: `buildMappedReferenceExpression` obtém o valor de `formatValue`, o mesmo literal das colunas. Só tabelas com chave primária inteira de uma coluna entram no mapa, então o valor de `old_id` é um inteiro e nunca contém `\` nem `'`. Se um valor fora do formato de número aparecer, ele sai entre aspas e escapado como qualquer outro texto.

**Ida e volta (round-trip)**

Além da tabela, cada dialeto tem um teste que escreve um conjunto de valores, lê de volta e compara com o original. O conjunto mínimo (cada item é um valor de teste): `abc`, `C:\temp`, `a\\b`, `abc\`, `\`, `it's`, `C:\temp\'x`, `'; DROP TABLE x;--`, `\'; DROP TABLE x;--`, `a<NUL>b` (menos no postgres e na ida e volta do sqlite), `l1<LF>l2`, `l1<CR><LF>l2`, `a<TAB>b`, `a<SUB>b`, `say "hi"`, `ação 日本 😀`, `\n` (barra e `n`) e o texto vazio.

- **sqlite:** abrir um SQLite em memória (o `modernc.org/sqlite` que o projeto já usa), executar `SELECT <literal>` com o literal que `formatValue("sqlite", "text", ...)` devolve e conferir que o valor lido é idêntico ao original. Vale também para `formatValue("sqlite", ...)` dentro de um `INSERT` seguido de `SELECT`. O item com NUL **não entra** na ida e volta do sqlite: o driver `modernc.org/sqlite` corta o SQL no byte NUL (`unrecognized token`), mesmo com o literal correto. O teste desse item é pulado (`t.Skip`) com esse motivo. A saída do `formatValue` para NUL no sqlite continua a da tabela (byte inalterado).
- **mysql e postgres:** não há banco nos testes unitários. Cada teste escreve um decodificador pequeno que aplica a regra de leitura do dialeto ao literal e compara com o original:
  - decodificador do mysql: tira as aspas externas; lê da esquerda para a direita; `''` vale `'`; `\\` vale `\`; `\0` vale NUL; `\n` vale LF; `\r` vale CR; `\Z` vale Ctrl-Z; `\'` vale `'`; os demais caracteres valem eles mesmos. Uma aspa simples sozinha no meio do conteúdo, ou uma barra no fim sem caractere seguinte, falha o teste.
  - decodificador do postgres: tira as aspas externas e troca `''` por `'`; a barra não tem significado. Uma aspa simples sozinha no meio do conteúdo falha o teste.
- Em todos, o teste também confere que **nenhuma aspa simples sozinha** aparece dentro do literal (toda `'` do conteúdo vem em par), o que garante que o texto não consegue fechar o literal.
- Teste negativo (mysql): o decodificador do mysql aplicado ao literal que o código antigo geraria para `C:\temp\'x` (`'C:\temp\''x'`) **não** devolve o valor original. Isso mostra que o teste pega o erro.

### Teste de integração (opcional)

Exportar de um SQLite em memória uma tabela com colunas `INTEGER`, `BOOLEAN`, `BLOB`, `TEXT` e `TIMESTAMP`, com `--output-driver` igual a `postgres`, e conferir que o `INSERT` contém `TRUE`, `'\x...'` e o texto entre aspas, nessa ordem de colunas. Os testes de integração existentes em `internal/export` continuam passando sem mudança de saída para `INTEGER`, `TEXT` e `NULL`.
