// Package testutil provê um harness de testes de integração com repositórios
// git reais em diretórios temporários (um bare repo como origin) e um mock do gh.
package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mathewsjts/medcof-git-flow/internal/git"
)

// Repo é um clone de trabalho ligado a um bare origin, com identidade git fixa.
type Repo struct {
	T      *testing.T
	Dir    string // clone de trabalho (onde o flow roda)
	Origin string // caminho do bare origin
	G      *git.Git
}

// NewRepo cria origin (bare) + clone, com commit inicial em main e as branches
// de ambiente dev e hom (apontando para main). envs são branches extras a criar.
func NewRepo(t *testing.T, envBranches ...string) *Repo {
	t.Helper()
	base := t.TempDir()
	origin := filepath.Join(base, "origin.git")
	runGit(t, base, "init", "--bare", "-b", "main", origin)

	work := filepath.Join(base, "work")
	runGit(t, base, "clone", origin, work)
	r := &Repo{T: t, Dir: work, Origin: origin, G: git.New(work)}

	r.config("user.email", "dev@example.com")
	r.config("user.name", "Dev Example")
	r.config("commit.gpgsign", "false")
	r.config("rerere.enabled", "true")
	// Remote origin SSH-like para exercitar a normalização quando necessário.
	// (mantemos a URL real do clone para permitir push.)

	r.WriteFile("README.md", "# fixture\n")
	r.commitAll("commit inicial")
	r.Push("main")

	if len(envBranches) == 0 {
		envBranches = []string{"dev", "hom"}
	}
	for _, b := range envBranches {
		r.runOnWork("branch", b, "main")
		r.Push(b)
	}
	// Volta para main como branch default do clone.
	r.runOnWork("checkout", "main")
	return r
}

func (r *Repo) config(k, v string) {
	r.T.Helper()
	runGit(r.T, "", "-C", r.Dir, "config", k, v)
}

func (r *Repo) runOnWork(args ...string) string {
	r.T.Helper()
	return runGit(r.T, "", append([]string{"-C", r.Dir}, args...)...)
}

// WriteFile escreve/atualiza um arquivo no clone.
func (r *Repo) WriteFile(rel, content string) {
	r.T.Helper()
	p := filepath.Join(r.Dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		r.T.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		r.T.Fatalf("write %s: %v", rel, err)
	}
}

func (r *Repo) commitAll(msg string) string {
	r.T.Helper()
	r.runOnWork("add", "-A")
	r.runOnWork("commit", "-m", msg)
	return r.SHA("HEAD")
}

// CommitFile escreve um arquivo e faz commit; devolve o SHA.
func (r *Repo) CommitFile(rel, content, msg string) string {
	r.T.Helper()
	r.WriteFile(rel, content)
	return r.commitAll(msg)
}

// Checkout troca de branch; create=true cria a branch a partir do HEAD atual.
func (r *Repo) Checkout(branch string, create bool) {
	r.T.Helper()
	if create {
		r.runOnWork("checkout", "-b", branch)
	} else {
		r.runOnWork("checkout", branch)
	}
}

// Push envia uma branch local para o origin.
func (r *Repo) Push(branch string) {
	r.T.Helper()
	r.runOnWork("push", "-u", "origin", branch)
}

// SHA resolve uma revisão no clone.
func (r *Repo) SHA(rev string) string {
	r.T.Helper()
	return strings.TrimSpace(r.runOnWork("rev-parse", rev))
}

// Fetch atualiza o clone a partir do origin.
func (r *Repo) Fetch() { r.T.Helper(); r.runOnWork("fetch", "--prune", "origin") }

// FileOnBranch devolve o conteúdo de um arquivo numa branch do origin.
func (r *Repo) FileOnOrigin(branch, rel string) (string, bool) {
	r.T.Helper()
	r.Fetch()
	out, err := r.G.Run("show", "origin/"+branch+":"+filepath.ToSlash(rel))
	if err != nil {
		return "", false
	}
	return out, true
}

// LogOnOrigin devolve os assuntos (subjects) dos commits de origin/branch, do
// mais novo ao mais antigo, limitado a n (0 = todos).
func (r *Repo) BodiesOnOrigin(branch string) []git.LogCommit {
	r.T.Helper()
	r.Fetch()
	commits, err := r.G.Log("origin/"+branch, false)
	if err != nil {
		r.T.Fatalf("log origin/%s: %v", branch, err)
	}
	return commits
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(),
		"LC_ALL=C",
		"GIT_AUTHOR_NAME=Dev Example", "GIT_AUTHOR_EMAIL=dev@example.com",
		"GIT_COMMITTER_NAME=Dev Example", "GIT_COMMITTER_EMAIL=dev@example.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}
