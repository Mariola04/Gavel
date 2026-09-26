# Projeto: Gavel — avaliador automático de funções Go

Constrói um protótipo de avaliador automático de programas escrito em Go, que avalia submissões em Go. É um projeto académico (grupo de até 3 alunos). Prioridades, por ordem: **correção**, **simplicidade**, **fail-fast** (rejeitar o mais cedo e barato possível), **boas práticas Go**. Não adiciones complexidade que não esteja pedida.

O avaliador avalia apenas **funções**: chama a função do aluno com argumentos e compara o valor devolvido. Não há avaliação de I/O (stdin/stdout).

## Restrições gerais

- Go 1.22+ (usa o routing de `net/http` com padrões `GET /exercises/{id}`).
- **Apenas stdlib** no binário. Nada de frameworks web, nada de bibliotecas de CLI. Se achares que precisas de uma dependência, pergunta primeiro.
- Identificadores e comentários de código em inglês; textos para o utilizador (CLI, web, mensagens de relatório, README) em português de Portugal.
- Servidor apenas em `localhost:8080` por omissão. Sem autenticação, sem base de dados.
- Trabalha por fases (abaixo). No fim de cada fase corre `make check` e só avanças quando passar.

## Estrutura do repositório

```
gavel/
├── cmd/gavel/main.go          # entrypoint: subcomandos
├── internal/
│   ├── exercise/              # carregar e validar exercícios do disco
│   ├── exam/                  # modelos de prova, sorteio, tentativas, pontuação
│   ├── engine/                # pipeline de avaliação
│   │   ├── static.go          # verificações estáticas (AST, gofmt, complexidade)
│   │   ├── tools.go           # go vet, gosec (ferramentas externas)
│   │   ├── harness.go         # geração do main.go via text/template
│   │   ├── runner.go          # build + execução com timeout/sandbox
│   │   └── report.go          # tipos do relatório + veredito
│   ├── store/                 # guardar/ler relatórios e tentativas em JSON
│   ├── server/                # handlers HTTP + middleware CORS
│   └── client/                # interface Client: LocalClient e RemoteClient
├── web/                       # index.html, style.css, app.js (embed.FS)
├── data/
│   ├── exercises/<id>/exercise.json
│   ├── exercises/<id>/solution.go   # solução de referência
│   ├── exams/<id>.json              # modelos de prova
│   ├── attempts/                    # gerado em runtime (no .gitignore)
│   └── reports/                     # gerado em runtime (no .gitignore)
├── testdata/submissions/      # submissões de teste (boas e maliciosas)
├── Makefile
├── .golangci.yml
└── README.md
```

## Formato de exercício

Um exercício = uma diretoria. O nome da diretoria é o `id`.

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
    { "input": [1], "output": 1 },
    { "input": [5], "output": 120 }
  ]
}
```

- `difficulty` é um de `easy`, `medium`, `hard` (em português na UI: fácil, médio, difícil).
- Pontos por nível: `easy` = 1, `medium` = 2, `hard` = 3. Define-os como constantes num único sítio.
- `input` é sempre um array (um elemento por parâmetro).
- Apenas **um** valor de retorno (sem `error`). Tipos suportados: tudo o que for serializável em JSON (int, float64, string, bool, slices, maps com chave string).
- Ao carregar, valida: campos obrigatórios, `difficulty` válida, `len(input) == len(params)`, `function` exportada.

## Exercícios a criar

1. Cria 5 exercícios de raiz com solução de referência: `factorial`, `fibonacci`, `reverse` (string), `sum` ([]int → int), `palindrome` (string → bool).
2. Converte exercícios de `exercism/problem-specifications` (licença MIT) que sejam funções puras com um único retorno. Candidatos: leap, hamming, raindrops, isogram, pangram, collatz-conjecture, darts, grains, armstrong-numbers, acronym, roman-numerals, run-length-encoding, matching-brackets, nth-prime, sieve, change.
   - Usa os inputs e outputs do `canonical-data.json` de cada um como testes.
   - Escreve as soluções de referência de raiz.
   - Se um caso canónico esperar um erro, omite-o.
3. Classifica cada exercício por dificuldade, de forma a haver **pelo menos 4 exercícios por nível**.
4. No README, indica a origem dos exercícios convertidos e a licença MIT.

## Provas (níveis de teste)

Uma prova é um modelo que define quantos exercícios de cada nível são sorteados. Ficheiro `data/exams/<id>.json`:

```json
{
  "title": "Teste fácil",
  "description": "2 exercícios fáceis e 1 médio.",
  "composition": { "easy": 2, "medium": 1 }
}
```

- Cria 3 modelos:
  - `easy`: 2 easy + 1 medium
  - `medium`: 1 easy + 2 medium + 1 hard
  - `hard`: 1 medium + 3 hard
- Ao arrancar, valida os modelos: níveis válidos, contagens > 0, e exercícios suficientes de cada nível no repositório. Se algum falhar, o erro deve ser claro e o arranque falha.
- **Iniciar uma prova** cria uma tentativa:
  - sorteia os exercícios com `math/rand/v2` e guarda a seed na tentativa;
  - não repete exercícios dentro da mesma tentativa;
  - guarda a tentativa em `data/attempts/<attempt_id>.json`, com `attempt_id`, `exam_id`, `started_at`, `seed` e a lista de `exercise_id`.
- As submissões podem levar um `attempt_id` opcional. Se levarem, o exercício tem de pertencer à tentativa; caso contrário devolve 400. A submissão fica associada à tentativa.
- **Pontuação da tentativa:**
  - por exercício, conta a melhor submissão (maior `score`);
  - pontos obtidos = `score × pontos do nível`;
  - total = soma dos pontos obtidos / soma dos pontos possíveis;
  - mostra também o detalhe por exercício (nível, melhor score, pontos).
- Mantém o modo de prática livre: submeter qualquer exercício sem tentativa continua a funcionar.

## Formato da submissão

- Ficheiro único com `package solution`, que define a função pedida (exportada).
- Tamanho máximo: 64 KB.

## Pipeline de avaliação (fail-fast)

As etapas correm por esta ordem. Uma etapa **bloqueante** que falhe termina a avaliação e o relatório indica em que etapa parou. Etapas não bloqueantes geram avisos e contam para o relatório, mas não impedem a execução.

| # | Etapa | Como | Bloqueante |
|---|-------|------|------------|
| 1 | Limites | tamanho do código, exercício existe, exercício pertence à tentativa (se houver) | sim |
| 2 | Parse | `go/parser` | sim |
| 3 | Regras AST | `package solution`; imports só da allowlist; função existe com assinatura igual a `params`/`returns`; proibido `func main`, `func init`, `import "C"`, diretivas `//go:` | sim |
| 4 | Formatação | `go/format.Source` comparado com o original | não (aviso) |
| 5 | Complexidade | complexidade ciclomática por função, calculada no AST (if, for, case, &&, \|\|) — sem dependência externa | não (aviso se > 10) |
| 6 | `go vet` | processo externo sobre o módulo temporário | sim |
| 7 | `gosec` | processo externo; se não estiver no PATH, marca a etapa como `skipped` | sim só para severidade HIGH |
| 8 | Build | `go build` com `CGO_ENABLED=0` | sim |
| 9 | Execução | corre o binário com os testes | — |

**Allowlist de imports:** `fmt`, `math`, `strings`, `strconv`, `sort`, `slices`, `maps`, `unicode`, `unicode/utf8`, `errors`. Tudo o resto é rejeitado com mensagem clara ("import não permitido: os").

As etapas 4 e 5 correm em paralelo, e as etapas 6 e 7 também (goroutines + `sync.WaitGroup`). O resto é sequencial.

## Harness e sandbox

- Cria um módulo temporário (`os.MkdirTemp`):
```
  go.mod                  -> module sandbox
  solution/solution.go    -> código do aluno
  main.go                 -> harness gerado
```
  O harness está em `package main` e importa `sandbox/solution`. Assim o código do aluno **não vê** os identificadores do avaliador.
- O harness é gerado com `text/template` a partir de `params`/`returns`:
  - lê os testes em JSON do **stdin**;
  - para cada teste, faz `json.Unmarshal` de cada argumento para o tipo certo e chama `solution.<Function>(...)` numa goroutine com `recover()`;
  - usa um `select` com timer (`timeout_ms` do exercício) por teste;
  - escreve cada resultado numa linha com o prefixo `@@RESULT@@ ` seguido de JSON (`index`, `got`, `error`, `duration_ms`). As linhas sem prefixo são ignoradas, porque o aluno pode usar `fmt`.
  - em timeout, reporta o teste como `timeout` e termina com `os.Exit` (a goroutine presa morre com o processo).
- O runner:
  - usa `exec.CommandContext` com timeout global = soma dos timeouts + margem;
  - mata o grupo de processos;
  - arranca com um ambiente limpo (apenas `PATH` mínimo);
  - usa a diretoria temporária como `cwd`;
  - limita o stdout a 1 MB.
- `GOCACHE` partilhado numa diretoria do Gavel, para que builds repetidos sejam rápidos.
- Sandbox configurável com a flag `--sandbox=auto|firejail|none`. Em `auto`, usa Firejail se existir (`--quiet --net=none --private=<tmpdir> --noroot`); caso contrário usa `none` e regista um aviso no relatório.
- Comparação: o engine faz unmarshal de `got` e de `output` para `any` e compara com `reflect.DeepEqual`.
- Remove sempre a diretoria temporária (`defer os.RemoveAll`).
- Limita as avaliações concorrentes a 2 com um semáforo (canal com buffer).

## Relatório de submissão (JSON)

Guardado em `data/reports/<submission_id>.json`. O `submission_id` é um timestamp mais 6 caracteres hex aleatórios.

```json
{
  "submission_id": "20260926T110200-a1b2c3",
  "exercise_id": "factorial",
  "attempt_id": "",
  "submitted_at": "2026-09-26T11:02:00Z",
  "code": "...",
  "static": {
    "passed": true,
    "checks": [
      { "name": "gofmt", "status": "warning", "messages": ["código não formatado"] }
    ]
  },
  "dynamic": {
    "executed": true,
    "tests": [
      { "input": [0], "expected": 1, "got": 1, "passed": true, "duration_ms": 0 }
    ]
  },
  "summary": {
    "verdict": "passed",
    "passed": 6,
    "total": 6,
    "score": 1.0,
    "stopped_at": ""
  }
}
```

- Valores possíveis de `status`: `ok`, `warning`, `error`, `skipped`.
- Valores possíveis de `verdict`: `passed`, `failed`, `rejected` (estática bloqueante), `compile_error`, `timeout`, `internal_error`.
- O `hint` só aparece nos testes falhados.

## Servidor HTTP (stdlib)

Comando: `gavel serve [-addr localhost:8080]`.

| Método | Rota | Descrição |
|--------|------|-----------|
| GET | `/api/exercises` | lista (id, title, description, difficulty); aceita `?difficulty=easy` |
| GET | `/api/exercises/{id}` | detalhe, **sem** `output` dos testes |
| GET | `/api/exams` | lista de modelos de prova |
| POST | `/api/exams/{id}/attempts` | inicia tentativa → `attempt_id` + exercícios sorteados |
| GET | `/api/attempts/{id}` | tentativa com pontuação atual e detalhe por exercício |
| POST | `/api/submissions` | body `{"exercise_id": "...", "code": "...", "attempt_id": "opcional"}` → relatório completo (síncrono) |
| GET | `/api/submissions/{id}` | relatório guardado |
| GET | `/` | ficheiros de `web/` via `embed.FS` |

- Middleware CORS (`Access-Control-Allow-Origin: *`) e tratamento de `OPTIONS`.
- `http.MaxBytesReader` no POST.
- Erros sempre em JSON `{"error": "..."}` com o status correto (400, 404, 413, 500).
- Timeouts no `http.Server` (`ReadHeaderTimeout`, etc.).
- Log de cada pedido com `log/slog`.

## CLI

Um único binário com subcomandos (só `flag` da stdlib):

```
gavel exercises [-difficulty easy]          # tabela: Id | Nível | Título | Descrição
gavel show <id>                             # detalhe do exercício
gavel exams                                 # lista modelos de prova
gavel start <exam_id>                       # inicia tentativa, mostra attempt_id e exercícios
gavel submit [-attempt <id>] <exercise_id> <ficheiro.go>
gavel attempt <attempt_id>                  # pontuação da tentativa
gavel report <submission_id>                # relatório guardado
gavel serve [-addr ...] [--sandbox ...]
```

- Define uma interface `Client` com os métodos necessários (exercícios, provas, tentativas, submissões, relatórios).
  - `LocalClient` usa o engine e o disco diretamente.
  - `RemoteClient` usa HTTP.
  - Se a variável de ambiente `SERVER` estiver definida (ex.: `export SERVER=http://localhost:8080`), usa o `RemoteClient`.
- Os mesmos comandos funcionam em local e em remoto.
- `submit` termina com exit code 0 se o veredito for `passed`, e 1 caso contrário.
- Imprime as tabelas com `text/tabwriter`.

## Interface web

Uma única página em HTML/CSS/JS puro (sem framework, sem build step), com JavaScript a chamar a API via `fetch`. Tem dois modos:

1. **Prática:**
   - lista de exercícios com etiqueta de nível e filtro por nível;
   - ao clicar, mostra a descrição, a assinatura esperada e uma `<textarea>` com um esqueleto `package solution` + a função vazia;
   - um botão "Submeter" que mostra o relatório: veredito destacado, verificações estáticas e a tabela de testes com esperado/obtido/hint.
2. **Prova:**
   - o aluno escolhe um modelo de prova e inicia;
   - vê os exercícios sorteados, submete cada um com o mesmo editor e relatório;
   - vê a pontuação total e o detalhe por exercício, que se atualiza após cada submissão;
   - guarda o `attempt_id` na página e mostra-o, para o aluno poder retomar a prova.

CSS simples e legível, com cores distintas por nível. Sem bibliotecas externas.

## Qualidade do próprio projeto

- `Makefile` com os alvos:
  - `fmt` (gofmt -w)
  - `vet`
  - `lint` (golangci-lint com: govet, staticcheck, errcheck, gosec, gofmt, gocyclo, ineffassign, unused)
  - `test` (`go test -race ./...`)
  - `check` (fmt-check + vet + lint + test)
  - `run`
  - `build`
- `.golangci.yml` mínimo com esses linters.
- Erros com contexto (`fmt.Errorf("...: %w", err)`); nada de `panic` fora de `main`.
- `context.Context` propagado do handler/CLI até ao runner.

## Testes (obrigatórios)

- **Unitários:**
  - regras AST: import proibido, assinatura errada, package errado, `init`, `main`;
  - cálculo da complexidade;
  - geração do harness (compila para vários tipos);
  - comparação de resultados;
  - validação de exercícios e de modelos de prova;
  - sorteio: respeita a composição, não repete exercícios, e é reprodutível com a mesma seed;
  - pontuação da tentativa: melhor submissão por exercício, pesos por nível.
- **Integração do engine** com `testdata/submissions/`. Casos: solução correta, resposta errada, erro de sintaxe, erro de tipos, `import "os"`, `import "net/http"`, loop infinito (→ timeout), `panic`, código não formatado (→ passa com aviso), uso de `fmt.Println` (não pode partir o harness).
- **Validação dos exercícios:** todas as soluções de referência em `data/exercises/*/solution.go` passam os seus próprios testes.
- **Servidor:** testes com `httptest` para cada rota, incluindo 404, body demasiado grande e submissão com exercício fora da tentativa.

## Fases

1. Estrutura, `go.mod`, Makefile, golangci, carregamento de exercícios + comandos `exercises`/`show` locais.
2. Engine: etapas estáticas 1–5 com testes.
3. Engine: vet/gosec, harness, build, execução, relatório, store + comandos `submit`/`report` locais.
4. Exercícios: os 5 de raiz + os convertidos do Exercism, classificados por nível e validados pelas soluções de referência.
5. Provas: modelos, sorteio, tentativas, pontuação + comandos `exams`/`start`/`attempt`.
6. Servidor HTTP + `RemoteClient` via `SERVER`.
7. Interface web (prática e prova).
8. README em português com:
   - requisitos (Go, opcional gosec e firejail);
   - como correr;
   - formato de exercício e de prova, e como adicionar cada um;
   - API;
   - origem e licença dos exercícios;
   - explicação curta das decisões de segurança e das suas limitações (sem firejail não há isolamento de rede/filesystem garantido; a allowlist de imports é a principal defesa).

No fim, mostra-me um resumo do que foi feito, como correr, e qualquer limitação conhecida.