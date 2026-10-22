package flow

import (
	"strings"
	"testing"

	"github.com/mathewsjts/medcof-git-flow/internal/testutil"
)

// conflictingSetup cria em origin/dev um commit de outra pessoa que conflita
// com o segundo dos três commits de feature/x.
func conflictingSetup(t *testing.T) *testutil.Repo {
	t.Helper()
	r := testutil.NewRepo(t)
	r.Checkout("dev", false)
	r.CommitFile("src/a.txt", "dev\n", "feat: outra pessoa")
	r.Push("dev")
	r.Checkout("main", false)
	featureBranch(r,
		[2]string{"src/b.txt", "b1\n"},
		[2]string{"src/a.txt", "a1\n"},
		[2]string{"src/c.txt", "c1\n"},
	)
	return r
}

func TestDevConflictOpensInWorkingTreeAndRerunPushes(t *testing.T) {
	r := conflictingSetup(t)
	app, out := testApp(t, r, testutil.NewGHMock())
	devBefore := r.SHA("origin/dev")

	err := app.Dev("feature/x")
	if err == nil || !strings.Contains(err.Error(), "flow dev feature/x") {
		t.Fatalf("esperava conflito com o comando para re-rodar, veio: %v", err)
	}
	if b := currentBranch(t, r); b != "dev" {
		t.Fatalf("conflito deve deixar o working tree em dev, está em %s", b)
	}
	if !r.G.CherryPickInProgress() {
		t.Fatal("esperava cherry-pick em andamento")
	}
	files, _ := r.G.UnmergedFiles()
	if len(files) != 1 || files[0] != "src/a.txt" {
		t.Fatalf("arquivos em conflito = %v", files)
	}
	if !strings.Contains(out.String(), "git cherry-pick --continue") {
		t.Fatalf("esperava instruções de resolução:\n%s", out)
	}
	r.Fetch()
	if r.SHA("origin/dev") != devBefore {
		t.Fatal("nada deve ser enviado antes da resolução")
	}

	// Re-rodar sem terminar o cherry-pick é recusado.
	if err := app.Dev("feature/x"); err == nil || !strings.Contains(err.Error(), "cherry-pick em andamento") {
		t.Fatalf("esperava recusa por cherry-pick em andamento, veio: %v", err)
	}

	// Resolve; o --continue segue a sequência e aplica c.
	r.WriteFile("src/a.txt", "resolvido\n")
	r.RunGit("add", "src/a.txt")
	r.RunGit("cherry-pick", "--continue")

	out.Reset()
	if err := app.Dev("feature/x"); err != nil {
		t.Fatalf("Dev pós-resolução: %v", err)
	}
	if !strings.Contains(out.String(), "resolução enviados") {
		t.Fatalf("esperava envio da resolução:\n%s", out)
	}
	envHasFile(t, r, "dev", "src/a.txt", "resolvido")
	envHasFile(t, r, "dev", "src/b.txt", "b1")
	envHasFile(t, r, "dev", "src/c.txt", "c1")
	if n := pickedCount(t, r, "dev"); n != 3 {
		t.Fatalf("esperava 3 commits com 'cherry picked from' em dev, veio %d", n)
	}

	out.Reset()
	if err := app.Dev("feature/x"); err != nil {
		t.Fatalf("Dev final: %v", err)
	}
	if !strings.Contains(out.String(), "nada a aplicar") {
		t.Fatalf("esperava nada a aplicar:\n%s", out)
	}
}

func TestDevConflictResolvedAndBackOnFeature(t *testing.T) {
	r := conflictingSetup(t)
	app, _ := testApp(t, r, testutil.NewGHMock())
	_ = app.Dev("feature/x")

	// Resolve, volta para a feature e re-roda dela, sem argumento.
	r.WriteFile("src/a.txt", "resolvido\n")
	r.RunGit("add", "src/a.txt")
	r.RunGit("cherry-pick", "--continue")
	r.Checkout("feature/x", false)

	if err := app.Dev(""); err != nil {
		t.Fatalf("Dev: %v", err)
	}
	envHasFile(t, r, "dev", "src/a.txt", "resolvido")
	envHasFile(t, r, "dev", "src/c.txt", "c1")
}

func TestDevConflictRefusesDirtyWorkingTree(t *testing.T) {
	r := conflictingSetup(t)
	r.WriteFile("README.md", "mudança local\n")
	app, _ := testApp(t, r, testutil.NewGHMock())

	err := app.Dev("feature/x")
	if err == nil || !strings.Contains(err.Error(), "git stash") {
		t.Fatalf("esperava recusa por working tree sujo, veio: %v", err)
	}
	if b := currentBranch(t, r); b != "main" {
		t.Fatalf("não deveria trocar de branch; está em %s", b)
	}
}

func TestDevConflictDiscardsStaleLocalEnv(t *testing.T) {
	r := conflictingSetup(t)
	// dev local com commit que saiu de origin/dev mas segue em outra branch remota.
	r.Checkout("dev", false)
	r.RunGit("pull", "origin", "dev")
	r.CommitFile("src/z.txt", "z\n", "chore: reescrito em origin")
	r.Push("dev")
	r.RunGit("push", "origin", "dev:old")
	r.RunGit("push", "-f", "origin", "dev~1:dev")
	r.Checkout("main", false)
	app, _ := testApp(t, r, testutil.NewGHMock())

	err := app.Dev("feature/x")
	if err == nil || !strings.Contains(err.Error(), "conflito aberto em dev") {
		t.Fatalf("esperava conflito aberto, veio: %v", err)
	}
	if b := strings.TrimSpace(r.RunGit("branch", "--list", "flow-backup/*")); b != "" {
		t.Fatalf("não deveria criar backup de commits que estão no origin: %s", b)
	}
	if got, want := r.SHA("dev~1"), r.SHA("origin/dev"); got != want {
		t.Fatalf("dev deveria partir de origin/dev (%s), parte de %s", want, got)
	}
}

func TestDevConflictBacksUpLocalOnlyEnvCommits(t *testing.T) {
	r := conflictingSetup(t)
	r.Checkout("dev", false)
	r.RunGit("pull", "origin", "dev")
	local := r.CommitFile("src/z.txt", "z\n", "chore: local não enviado")
	r.Checkout("main", false)
	app, out := testApp(t, r, testutil.NewGHMock())

	err := app.Dev("feature/x")
	if err == nil || !strings.Contains(err.Error(), "conflito aberto em dev") {
		t.Fatalf("esperava conflito aberto, veio: %v", err)
	}
	backup := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(r.RunGit("branch", "--list", "flow-backup/dev-*")), "*"))
	if backup == "" || r.SHA(backup) != local {
		t.Fatalf("esperava backup apontando para %s, veio %q", local, backup)
	}
	if !strings.Contains(out.String(), backup) {
		t.Fatalf("a saída deveria citar o backup %s:\n%s", backup, out.String())
	}
}
