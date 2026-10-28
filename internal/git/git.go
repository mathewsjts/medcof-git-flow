// Package git é um adaptador fino sobre o executável `git`, invocado via
// os/exec sem shell. Todas as operações recebem o diretório de trabalho
// explicitamente.
package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Git executa comandos git num diretório de trabalho fixo (git -C dir).
type Git struct {
	dir string
}

// New cria um runner git para o diretório informado. dir vazio usa o CWD.
func New(dir string) *Git { return &Git{dir: dir} }

// Dir devolve o diretório de trabalho do runner.
func (g *Git) Dir() string { return g.dir }

// At devolve um novo runner para outro diretório (ex.: worktree), sem mutar o atual.
func (g *Git) At(dir string) *Git { return &Git{dir: dir} }

// CmdError carrega o stderr do git para diagnóstico/classificação.
type CmdError struct {
	Args   []string
	Stderr string
	Err    error
}

func (e *CmdError) Error() string {
	msg := strings.TrimSpace(e.Stderr)
	if msg == "" {
		msg = e.Err.Error()
	}
	return fmt.Sprintf("git %s: %s", strings.Join(e.Args, " "), msg)
}

func (e *CmdError) Unwrap() error { return e.Err }

// Run executa `git args...` e devolve o stdout (com CRLF normalizado e trim final).
func (g *Git) Run(args ...string) (string, error) {
	full := args
	if g.dir != "" {
		full = append([]string{"-C", g.dir}, args...)
	}
	cmd := exec.Command("git", full...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// Saída estável e em inglês para parsing confiável, independente do locale.
	cmd.Env = append(os.Environ(), "LC_ALL=C", "GIT_TERMINAL_PROMPT=0")
	err := cmd.Run()
	out := normalize(stdout.String())
	if err != nil {
		return out, &CmdError{Args: args, Stderr: stderr.String(), Err: err}
	}
	return out, nil
}

// RunOK executa e descarta o stdout, devolvendo só o erro.
func (g *Git) RunOK(args ...string) error {
	_, err := g.Run(args...)
	return err
}

// normalize remove CRLF (Windows) e espaços em branco nas pontas.
func normalize(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.TrimRight(s, "\n ")
}

// Lines quebra a saída em linhas não vazias, já normalizada.
func Lines(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// Version devolve major e minor da versão do git instalado.
func (g *Git) Version() (major, minor int, err error) {
	out, err := g.Run("--version")
	if err != nil {
		return 0, 0, err
	}
	// "git version 2.53.0" (pode ter sufixos, ex.: ".windows.1").
	fields := strings.Fields(out)
	if len(fields) < 3 {
		return 0, 0, fmt.Errorf("saída inesperada de `git --version`: %q", out)
	}
	parts := strings.SplitN(fields[2], ".", 3)
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("versão do git inesperada: %q", fields[2])
	}
	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("versão do git inesperada: %q", fields[2])
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("versão do git inesperada: %q", fields[2])
	}
	return major, minor, nil
}

// AtLeast verifica se a versão instalada é >= major.minor.
func (g *Git) AtLeast(major, minor int) (bool, error) {
	ma, mi, err := g.Version()
	if err != nil {
		return false, err
	}
	if ma != major {
		return ma > major, nil
	}
	return mi >= minor, nil
}

// RemoteURL devolve a URL do remote origin.
func (g *Git) RemoteURL() (string, error) {
	return g.Run("remote", "get-url", "origin")
}

// CommonDir devolve o caminho absoluto do diretório .git comum (compartilhado
// entre worktrees), usado para guardar estado e worktrees do flow.
func (g *Git) CommonDir() (string, error) {
	return g.Run("rev-parse", "--path-format=absolute", "--git-common-dir")
}

// CurrentBranch devolve o nome da branch corrente (HEAD simbólico).
func (g *Git) CurrentBranch() (string, error) {
	return g.Run("rev-parse", "--abbrev-ref", "HEAD")
}
