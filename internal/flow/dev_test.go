package flow

import (
	"strings"
	"testing"

	"github.com/mathewsjts/medcof-git-flow/internal/testutil"
)

func TestDevByPRNumber(t *testing.T) {
	r := testutil.NewRepo(t)
	gh := testutil.NewGHMock()
	featureBranch(r, [2]string{"src/a.txt", "a1\n"}, [2]string{"src/b.txt", "b1\n"})
	openPR(gh, 123, "feature/x")
	app, _ := testApp(t, r, gh)

	if err := app.Dev("#123"); err != nil {
		t.Fatalf("Dev: %v", err)
	}
	envHasFile(t, r, "dev", "src/a.txt", "a1")
	envHasFile(t, r, "dev", "src/b.txt", "b1")
	if n := pickedCount(t, r, "dev"); n != 2 {
		t.Fatalf("esperava 2 commits com 'cherry picked from' em dev, veio %d", n)
	}
	if b := currentBranch(t, r); b != "main" {
		t.Fatalf("caminho feliz não deve mexer no working tree; branch atual = %s", b)
	}
}

func TestDevByBranchWithoutPR(t *testing.T) {
	r := testutil.NewRepo(t)
	featureBranch(r, [2]string{"src/a.txt", "a1\n"})
	app, out := testApp(t, r, testutil.NewGHMock())

	if err := app.Dev("feature/x"); err != nil {
		t.Fatalf("Dev: %v", err)
	}
	envHasFile(t, r, "dev", "src/a.txt", "a1")
	if strings.Contains(out.String(), "PR #") {
		t.Fatalf("branch sem PR não deveria citar PR:\n%s", out)
	}
}

func TestDevDefaultsToCurrentBranch(t *testing.T) {
	r := testutil.NewRepo(t)
	featureBranch(r, [2]string{"src/a.txt", "a1\n"})
	r.Checkout("feature/x", false)
	app, _ := testApp(t, r, testutil.NewGHMock())

	if err := app.Dev(""); err != nil {
		t.Fatalf("Dev: %v", err)
	}
	envHasFile(t, r, "dev", "src/a.txt", "a1")
}

func TestDevRejectsEnvBranchAsTarget(t *testing.T) {
	r := testutil.NewRepo(t)
	r.Checkout("dev", false)
	app, _ := testApp(t, r, testutil.NewGHMock())

	err := app.Dev("")
	if err == nil || !strings.Contains(err.Error(), "não é uma branch de feature") {
		t.Fatalf("esperava recusa de dev como alvo, veio: %v", err)
	}
}

func TestDevRequiresBranchOnOrigin(t *testing.T) {
	r := testutil.NewRepo(t)
	r.Checkout("feature/local", true)
	r.CommitFile("src/a.txt", "a1\n", "feat: a")
	app, _ := testApp(t, r, testutil.NewGHMock())

	err := app.Dev("")
	if err == nil || !strings.Contains(err.Error(), "git push -u origin feature/local") {
		t.Fatalf("esperava pedido de push, veio: %v", err)
	}
}

func TestDevWarnsUnpushedLocalCommits(t *testing.T) {
	r := testutil.NewRepo(t)
	featureBranch(r, [2]string{"src/a.txt", "a1\n"})
	r.Checkout("feature/x", false)
	r.CommitFile("src/b.txt", "b1\n", "feat: b (local)")
	app, out := testApp(t, r, testutil.NewGHMock())

	if err := app.Dev(""); err != nil {
		t.Fatalf("Dev: %v", err)
	}
	if !strings.Contains(out.String(), "não estão em origin") {
		t.Fatalf("esperava aviso de commits locais não enviados:\n%s", out)
	}
	if _, ok := r.FileOnOrigin("dev", "src/b.txt"); ok {
		t.Fatal("commit local não enviado não deveria ir para dev")
	}
}

func TestDevReapplyOnlyNew(t *testing.T) {
	r := testutil.NewRepo(t)
	featureBranch(r, [2]string{"src/a.txt", "a1\n"}, [2]string{"src/b.txt", "b1\n"})
	app, _ := testApp(t, r, testutil.NewGHMock())
	if err := app.Dev("feature/x"); err != nil {
		t.Fatalf("primeira Dev: %v", err)
	}

	// Novo commit na feature após o review.
	r.Checkout("feature/x", false)
	r.CommitFile("src/c.txt", "c1\n", "feat: c")
	r.Push("feature/x")
	r.Checkout("main", false)

	if err := app.Dev("feature/x"); err != nil {
		t.Fatalf("segunda Dev: %v", err)
	}
	envHasFile(t, r, "dev", "src/c.txt", "c1")
	if n := pickedCount(t, r, "dev"); n != 3 {
		t.Fatalf("esperava 3 commits aplicados em dev (a, b, c), veio %d", n)
	}
}

func TestDevSkipsAlreadyAppliedAfterRebase(t *testing.T) {
	r := testutil.NewRepo(t)
	featureBranch(r, [2]string{"src/a.txt", "a1\n"})
	app, _ := testApp(t, r, testutil.NewGHMock())
	if err := app.Dev("feature/x"); err != nil {
		t.Fatalf("Dev inicial: %v", err)
	}

	// main avança com um commit que não conflita; feature é rebaseada sobre ele
	// (SHAs mudam, patch-id permanece). Depois um commit genuinamente novo.
	r.Checkout("main", false)
	r.CommitFile("docs/readme2.md", "x\n", "chore: docs")
	r.Push("main")
	r.Checkout("feature/x", false)
	r.RunGit("rebase", "origin/main")
	r.CommitFile("src/d.txt", "d1\n", "feat: d")
	r.RunGit("push", "-f", "origin", "feature/x")
	r.Checkout("main", false)

	if err := app.Dev("feature/x"); err != nil {
		t.Fatalf("Dev pós-rebase: %v", err)
	}
	envHasFile(t, r, "dev", "src/d.txt", "d1")
	if n := pickedCount(t, r, "dev"); n != 2 {
		t.Fatalf("esperava 2 commits aplicados em dev (a+d), veio %d", n)
	}
}

func TestDevSkipsCommitsMergedIntoEnv(t *testing.T) {
	r := testutil.NewRepo(t)
	featureBranch(r, [2]string{"src/a.txt", "a1\n"})

	// Fluxo antigo: a feature foi mergeada em dev por PR.
	r.Checkout("dev", false)
	r.RunGit("merge", "--no-ff", "-m", "Merge feature/x into dev", "feature/x")
	r.Push("dev")
	r.Checkout("main", false)

	app, out := testApp(t, r, testutil.NewGHMock())
	if err := app.Dev("feature/x"); err != nil {
		t.Fatalf("Dev: %v", err)
	}
	if !strings.Contains(out.String(), "nada a aplicar") {
		t.Fatalf("esperava nada a aplicar:\n%s", out)
	}
}

func TestDevCarriesDependencyCommits(t *testing.T) {
	r := testutil.NewRepo(t)
	// feature/base parte de main; feature/x parte de feature/base.
	r.Checkout("feature/base", true)
	r.CommitFile("src/base.txt", "base\n", "feat: base")
	r.Push("feature/base")
	r.Checkout("feature/x", true)
	r.CommitFile("src/x.txt", "x\n", "feat: x")
	r.Push("feature/x")
	r.Checkout("main", false)

	app, _ := testApp(t, r, testutil.NewGHMock())
	if err := app.Dev("feature/x"); err != nil {
		t.Fatalf("Dev: %v", err)
	}
	envHasFile(t, r, "dev", "src/base.txt", "base")
	envHasFile(t, r, "dev", "src/x.txt", "x")
}

func TestDevRejectsMergeCommits(t *testing.T) {
	r := testutil.NewRepo(t)

	// feature com um merge commit no intervalo.
	r.Checkout("feature/x", true)
	r.CommitFile("src/a.txt", "a1\n", "feat: a")
	r.Checkout("side", true)
	r.CommitFile("src/s.txt", "s1\n", "feat: side")
	r.Checkout("feature/x", false)
	r.RunGit("merge", "--no-ff", "-m", "merge side", "side")
	r.Push("feature/x")
	r.Checkout("main", false)

	app, _ := testApp(t, r, testutil.NewGHMock())
	err := app.Dev("feature/x")
	if err == nil || !strings.Contains(err.Error(), "merge commits") {
		t.Fatalf("esperava recusa por merge commits, veio: %v", err)
	}
}
