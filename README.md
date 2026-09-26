# Gavel avaliador automático de funções Go

Protótipo académico de um avaliador automático que corrige submissões em Go.
Cada exercício pede **uma função**; o Gavel chama a função do aluno com os
argumentos de cada teste e compara o valor devolvido com o esperado. Não há
avaliação de I/O (stdin/stdout).

Tem três interfaces sobre a mesma lógica:

- **CLI** (`gavel`), que funciona em local ou contra um servidor remoto;
- **API HTTP** (`gavel serve`);
- **interface web** (HTML/CSS/JS puro, servida pelo próprio binário), com modo
  de prática e modo de prova.

## Requisitos

- **Go 1.22 ou superior** (obrigatório: o Gavel usa o `go` para compilar as submissões).
- [gosec](https://github.com/securego/gosec) (opcional): se não estiver no `PATH`,
  a etapa `gosec` é marcada como `skipped`.
  `go install github.com/securego/gosec/v2/cmd/gosec@latest`
- [Firejail](https://firejail.wordpress.com/) (opcional, Linux): isola a execução
  sem rede e com sistema de ficheiros privado.
- Para desenvolvimento: [golangci-lint](https://golangci-lint.run/) v1.x.

> O `Makefile` acrescenta `$(go env GOPATH)/bin` ao `PATH`, por isso ferramentas
> instaladas com `go install` são encontradas pelos alvos `make`. Fora do
> `make`, garanta que o `gosec` está no `PATH` se quiser essa etapa.

## Como correr

```sh
make build                     # gera bin/gavel
./bin/gavel serve              # servidor em http://localhost:8080
```

Abra <http://localhost:8080> no browser. Em alternativa, `make run` arranca o
servidor com `go run`.

Os comandos correm a partir da raiz do repositório (usam `./data`). Para usar
outra diretoria de dados, defina `GAVEL_DATA`.

### CLI

```
gavel exercises [-difficulty easy]          # tabela: Id | Nível | Título | Descrição
gavel show <id>                             # detalhe do exercício
gavel exams                                 # lista modelos de prova
gavel start <exam_id>                       # inicia tentativa, mostra attempt_id e exercícios
gavel submit [-attempt <id>] <exercise_id> <ficheiro.go>
gavel attempt <attempt_id>                  # pontuação da tentativa
gavel report <submission_id>                # relatório guardado
gavel serve [-addr localhost:8080] [-sandbox auto|firejail|none]
```

As flags vêm antes dos argumentos (`gavel submit -attempt X factorial sol.go`).
O `submit` termina com código 0 se o veredito for `passed` e 1 caso contrário.

Por omissão a CLI trabalha diretamente sobre o disco. Se a variável `SERVER`
estiver definida, os mesmos comandos usam a API remota:

```sh
export SERVER=http://localhost:8080
gavel exercises -difficulty hard
```

Exemplo de uma prova:

```sh
gavel start easy                          # mostra o attempt_id e os exercícios sorteados
gavel submit -attempt <attempt_id> leap leap.go
gavel attempt <attempt_id>                # pontuação atual
```

### Desenvolvimento

```sh
make fmt      # gofmt -w
make vet      # go vet
make lint     # golangci-lint
make test     # go test -race ./...
make check    # fmt-check + vet + lint + test
```

## Formato de exercício

Cada exercício é uma diretoria em `data/exercises/<id>/` (o nome da diretoria é
o `id`: minúsculas, dígitos e hífenes) com dois ficheiros:

- `exercise.json` — a definição;
- `solution.go` — a solução de referência, em `package solution`.

```json
{
  "title": "Fatorial",
  "description": "Defina a função Factorial(n int) int que devolve n!.",
  "difficulty": "easy",
  "function": "Factorial",
  "params": ["int"],
  "returns": "int",
  "timeout_ms": 1000,
  "tests": [
    { "input": [0], "output": 1, "hint": "Ver caso base" },
    { "input": [5], "output": 120 }
  ]
}
```

| Campo | Regras |
|-------|--------|
| `difficulty` | `easy` (1 ponto), `medium` (2 pontos) ou `hard` (3 pontos) |
| `function` | identificador exportado |
| `params`, `returns` | tipos serializáveis em JSON: tipos simples (`int`, `float64`, `string`, `bool`, …), slices e maps com chave `string`, em qualquer combinação. Um único valor de retorno. |
| `timeout_ms` | limite por teste, positivo |
| `tests[].input` | array com um valor por parâmetro |
| `tests[].hint` | opcional; só é mostrado quando o teste falha |

**Para adicionar um exercício:** crie a diretoria com os dois ficheiros e corra
`make test`. O teste `TestReferenceSolutions` passa a solução de referência por
todo o pipeline e falha se ela não passar os seus próprios testes. Os
exercícios são validados ao arrancar; um exercício inválido impede o arranque
com uma mensagem clara.

Na comparação, `got` e `output` são convertidos para JSON genérico e comparados
com `reflect.DeepEqual` (os números são comparados como `float64`). Um slice ou
map `nil` é considerado igual a `[]` ou `{}`.

## Formato de prova

Um modelo de prova é um ficheiro `data/exams/<id>.json`:

```json
{
  "title": "Teste fácil",
  "description": "2 exercícios fáceis e 1 médio.",
  "composition": { "easy": 2, "medium": 1 }
}
```

Modelos incluídos: `easy` (2 fáceis + 1 médio), `medium` (1 fácil + 2 médios +
1 difícil) e `hard` (1 médio + 3 difíceis).

**Para adicionar um modelo:** crie o ficheiro. Ao arrancar, o Gavel verifica que
os níveis são válidos, que as contagens são positivas e que há exercícios
suficientes de cada nível; se não, o arranque falha.

**Tentativas.** Iniciar uma prova sorteia os exercícios (`math/rand/v2`, sem
repetições) e guarda a tentativa em `data/attempts/<attempt_id>.json` com a
seed, o que torna o sorteio reprodutível. As submissões podem indicar um
`attempt_id`; nesse caso o exercício tem de pertencer à tentativa.

**Pontuação.** Por exercício conta a melhor submissão (maior `score`, que é a
fração de testes passados). Pontos obtidos = `score × pontos do nível`; o total
é a soma dos pontos obtidos a dividir pela soma dos pontos possíveis.

## Pipeline de avaliação

As etapas correm por esta ordem. Uma etapa bloqueante que falhe termina a
avaliação, e o relatório indica em `summary.stopped_at` onde parou.

| # | Etapa (`name`) | Bloqueante | Veredito se falhar |
|---|----------------|-----------|--------------------|
| 1 | `limits` — código até 64 KB | sim | `rejected` |
| 2 | `parse` — `go/parser` | sim | `rejected` |
| 3 | `ast` — `package solution`, imports da allowlist, assinatura, sem `main`/`init`/`import "C"`/diretivas `//go:` | sim | `rejected` |
| 4 | `gofmt` | não (aviso) | — |
| 5 | `complexity` — ciclomática > 10 | não (aviso) | — |
| 6 | `vet` — `go vet` | sim | `rejected` (ou `compile_error` se o código não compilar) |
| 7 | `gosec` | só severidade HIGH | `rejected` |
| 8 | `build` — `go build`, `CGO_ENABLED=0` | sim | `compile_error` |
| 9 | execução | — | `passed`, `failed` ou `timeout` |

As etapas 4 e 5 correm em paralelo, tal como as 6 e 7. Um exercício inexistente
ou fora da tentativa é recusado antes do pipeline (HTTP 404/400).

Imports permitidos: `fmt`, `math`, `strings`, `strconv`, `sort`, `slices`,
`maps`, `unicode`, `unicode/utf8`, `errors`.

## API

Todas as respostas são JSON. Os erros têm a forma `{"error": "..."}` com o
status adequado (400, 404, 413 ou 500). CORS está aberto
(`Access-Control-Allow-Origin: *`).

| Método | Rota | Descrição |
|--------|------|-----------|
| GET | `/api/exercises` | lista (`id`, `title`, `description`, `difficulty`); aceita `?difficulty=easy` |
| GET | `/api/exercises/{id}` | detalhe, com a assinatura e os inputs dos testes, **sem** os outputs |
| GET | `/api/exams` | modelos de prova |
| POST | `/api/exams/{id}/attempts` | inicia tentativa (201) → `attempt_id`, exercícios sorteados e pontuação |
| GET | `/api/attempts/{id}` | tentativa com a pontuação atual e o detalhe por exercício |
| POST | `/api/submissions` | body `{"exercise_id": "...", "code": "...", "attempt_id": "opcional"}` → relatório completo (síncrono) |
| GET | `/api/submissions/{id}` | relatório guardado |
| GET | `/` | interface web |

Exemplo:

```sh
curl -s localhost:8080/api/submissions \
  -d '{"exercise_id":"sum","code":"package solution\n\nfunc Sum(n []int) int {\n\tt := 0\n\tfor _, x := range n {\n\t\tt += x\n\t}\n\treturn t\n}\n"}'
```

O relatório tem `static.checks` (estado `ok`, `warning`, `error` ou `skipped`
por etapa), `dynamic.tests` (input, esperado, obtido, erro, duração e, nos
testes falhados, a dica) e `summary` (`verdict`, `passed`, `total`, `score`,
`stopped_at`). Os relatórios ficam em `data/reports/<submission_id>.json`.

## Segurança e limitações

Correr código de terceiros é perigoso. As defesas do Gavel, por ordem de
importância:

1. **Allowlist de imports (principal defesa).** Sem `os`, `net`, `syscall`,
   `unsafe`, `reflect` ou `C`, o código do aluno não consegue aceder a ficheiros,
   à rede nem a processos. Diretivas `//go:` (ex.: `go:linkname`, `go:embed`),
   `init` e `main` também são proibidas.
2. **Harness separado.** O código do aluno é compilado como o pacote
   `sandbox/solution`, importado por um `main` gerado; o aluno não vê os
   identificadores do avaliador. Antes de chamar a função, o harness redireciona
   `os.Stdout` para stderr, e cada linha de resultado leva um nonce aleatório
   passado por variável de ambiente, que o aluno não consegue ler. Assim, o que o
   aluno imprime não parte o harness nem forja resultados.
3. **Limites de recursos.** Timeout por teste (goroutine + `select`) e timeout
   global com morte de todo o grupo de processos; stdout e stderr limitados a
   1 MB; ambiente limpo (só `PATH` mínimo); diretoria temporária própria,
   apagada no fim; no máximo 2 avaliações em simultâneo.
4. **Ferramentas.** `go vet` e `gosec` (que ignora anotações `#nosec` do aluno);
   o `go` corre com `GOPROXY=off`, `GOTOOLCHAIN=local` e `CGO_ENABLED=0`.
5. **Firejail** (`--sandbox=auto|firejail|none`). Em `auto`, usa Firejail se
   existir (`--net=none --private=<tmp> --noroot`); se não existir, a execução
   corre sem isolamento e o relatório inclui um aviso (`sandbox`).

Limitações conhecidas:

- **Sem Firejail não há isolamento garantido de rede nem de sistema de ficheiros.**
  A segurança depende então da allowlist e do compilador. Uma falha no runtime
  do Go ou na allowlist pode permitir fugir ao controlo.
- Não há limite de memória: uma submissão pode alocar muita memória até o
  timeout a matar. Com Firejail pode acrescentar `--rlimit-as`.
- O Firejail não foi testado neste ambiente de desenvolvimento (não estava
  instalado); o modo `none` foi o exercitado nos testes.
- A regra G115 do `gosec` (conversão de inteiros com possível overflow, ex.:
  `uint64(n)` com `n int`) tem severidade HIGH e, por isso, bloqueia a
  submissão, tal como pedido na especificação.
- A pontuação de uma tentativa lê todos os relatórios guardados; é adequado para
  um protótipo, mas não escala para muitos milhares de submissões.
- Números são comparados como `float64`, portanto inteiros acima de 2^53 podem
  ser considerados iguais se diferirem em pouco.
- Não há autenticação: qualquer pessoa com acesso ao servidor pode ver qualquer
  tentativa ou relatório, se souber o id.

## Origem e licença dos exercícios

Os exercícios `factorial`, `fibonacci`, `reverse`, `sum` e `palindrome` foram
criados de raiz para este projeto.

Os restantes (`leap`, `hamming`, `raindrops`, `isogram`, `pangram`,
`collatz-conjecture`, `darts`, `grains`, `armstrong-numbers`, `acronym`,
`roman-numerals`, `run-length-encoding`, `matching-brackets`, `nth-prime`,
`sieve`, `change`) foram convertidos do repositório
[exercism/problem-specifications](https://github.com/exercism/problem-specifications),
distribuído sob a **licença MIT** (Copyright (c) Exercism; texto completo em
[`data/exercises/LICENSE-exercism.txt`](data/exercises/LICENSE-exercism.txt)).
Os testes vêm do `canonical-data.json` de cada exercício, com estas
adaptações:

- casos que esperam um erro foram omitidos;
- casos substituídos por outros (`reimplements`) foram omitidos;
- casos com inteiros que não cabem num `int` de 64 bits foram omitidos
  (`armstrong-numbers`);
- `grains` usa só a propriedade `square` e `run-length-encoding` só `encode`;
- as descrições foram escritas em português, e as soluções de referência de
  raiz.

As soluções de referência ficam dentro do módulo, por isso também são
compiladas e verificadas por `go vet` e `golangci-lint` em `make check`.
