// Command gavel is an automatic grader for Go functions.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"gavel/internal/client"
	"gavel/internal/engine"
)

const usage = `Utilização:
  gavel exercises [-difficulty easy|medium|hard]   lista os exercícios
  gavel show <id>                                  mostra um exercício
  gavel exams                                      lista os modelos de prova (usados pelo docente)
  gavel sessions                                   lista as provas abertas pelo docente
  gavel join -student <nome> <session_id>          entra numa prova (ou retoma a tentativa)
  gavel submit [-attempt <id>] [-student <nome>] <exercise_id> <ficheiro.go>
                                                   submete uma solução
  gavel attempt <attempt_id>                       mostra a pontuação de uma tentativa
  gavel report <submission_id>                     mostra um relatório guardado
  gavel serve [-addr localhost:8080] [-sandbox auto|firejail|none]
                                                   arranca o servidor HTTP

Variáveis de ambiente:
  SERVER      URL de um servidor Gavel (ex.: http://localhost:8080); se
              definida, os comandos usam o servidor em vez do disco local
  GAVEL_DATA  diretoria de dados (por omissão: data)
  GAVEL_ADMIN_PASSWORD
              palavra-passe da área de docente (serve); sem ela, fica desativada
`

// errUsage marks errors caused by invalid command-line usage.
var errUsage = errors.New("utilização inválida")

// errNotPassed makes submit exit with status 1 when the verdict is not
// "passed".
var errNotPassed = errors.New("a submissão não foi aprovada")

// command runs a subcommand with its arguments.
type command func(ctx context.Context, args []string, stdout, stderr io.Writer) error

var commands = map[string]command{
	"exercises": cmdExercises,
	"show":      cmdShow,
	"exams":     cmdExams,
	"sessions":  cmdSessions,
	"join":      cmdJoin,
	"submit":    cmdSubmit,
	"attempt":   cmdAttempt,
	"report":    cmdReport,
	"serve":     cmdServe,
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	if args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprint(stdout, usage)
		return 0
	}
	cmd, ok := commands[args[0]]
	if !ok {
		fmt.Fprintf(stderr, "comando desconhecido: %s\n\n%s", args[0], usage)
		return 2
	}
	err := cmd(ctx, args[1:], stdout, stderr)
	switch {
	case err == nil:
		return 0
	case errors.Is(err, errNotPassed):
		return 1
	case errors.Is(err, errUsage):
		fmt.Fprintf(stderr, "erro: %v\n\n%s", err, usage)
		return 2
	default:
		fmt.Fprintf(stderr, "erro: %v\n", err)
		return 1
	}
}

// newClient returns a RemoteClient when SERVER is set, and a LocalClient
// otherwise.
func newClient() (client.Client, error) {
	if url := os.Getenv("SERVER"); url != "" {
		return client.NewRemote(url), nil
	}
	return client.OpenLocal(dataDir(), engine.SandboxAuto)
}

func dataDir() string {
	if dir := os.Getenv("GAVEL_DATA"); dir != "" {
		return dir
	}
	return "data"
}
