package flow

import (
	"strings"
	"testing"

	"github.com/mathewsjts/medcof-git-flow/internal/github"
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

func TestDevByPRURL(t *testing.T) {
	r := testutil.NewRepo(t)
	gh := testutil.NewGHMock()
	featureBranch(r, [2]string{"src/a.txt", "a1\n"})
	openPR(gh, 70, "feature/x")
	app, _ := testApp(t, r, gh)

	if err := app.Dev("https://github.com/Test/App/pull/70/files"); err != nil {
		t.Fatalf("Dev: %v", err)
	}
	envHasFile(t, r, "dev", "src/a.txt", "a1")

	err := app.Dev("https://github.com/outro/repo/pull/70")
	if err == nil || !strings.Contains(err.Error(), "este repositório é test/app") {
		t.Fatalf("esperava recusa de PR de outro repo, veio: %v", err)
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

func TestDevSkipsMergeFromProduction(t *testing.T) {
	r := testutil.NewRepo(t)

	// feature atualizada com merge da main, que andou depois da criação.
	r.Checkout("feature/x", true)
	r.CommitFile("src/a.txt", "a1\n", "feat: a")
	r.Checkout("main", false)
	r.CommitFile("src/m.txt", "m1\n", "feat: main")
	r.Push("main")
	r.Checkout("feature/x", false)
	r.RunGit("merge", "--no-ff", "-m", "Merge branch 'main' into feature/x", "main")
	r.CommitFile("src/b.txt", "b1\n", "feat: b")
	r.Push("feature/x")
	r.Checkout("main", false)

	app, _ := testApp(t, r, testutil.NewGHMock())
	if err := app.Dev("feature/x"); err != nil {
		t.Fatalf("Dev: %v", err)
	}
	envHasFile(t, r, "dev", "src/a.txt", "a1")
	envHasFile(t, r, "dev", "src/b.txt", "b1")
	if got := pickedCount(t, r, "dev"); got != 2 {
		t.Fatalf("commits aplicados em dev = %d, queria 2", got)
	}
}

func TestDevAcceptsMergeOfOtherFeature(t *testing.T) {
	r := testutil.NewRepo(t)

	// feature com merge de outra feature, que vai junto como dependência.
	r.Checkout("feature/x", true)
	r.CommitFile("src/a.txt", "a1\n", "feat: a")
	r.Checkout("side", true)
	r.CommitFile("src/s.txt", "s1\n", "feat: side")
	r.Checkout("feature/x", false)
	r.RunGit("merge", "--no-ff", "-m", "merge side", "side")
	r.Push("feature/x")
	r.Checkout("main", false)

	app, _ := testApp(t, r, testutil.NewGHMock())
	if err := app.Dev("feature/x"); err != nil {
		t.Fatalf("Dev: %v", err)
	}
	envHasFile(t, r, "dev", "src/a.txt", "a1")
	envHasFile(t, r, "dev", "src/s.txt", "s1")
}

func TestDevAcceptsPullMerge(t *testing.T) {
	r := testutil.NewRepo(t)

	// `git pull` sem rebase: o remoto andou e a branch local fez merge dele.
	r.Checkout("feature/x", true)
	r.CommitFile("src/a.txt", "a1\n", "feat: a")
	r.Push("feature/x")
	r.CommitFile("src/b.txt", "b1\n", "feat: b")
	r.Push("feature/x")
	r.RunGit("reset", "-q", "--hard", "HEAD~1")
	r.CommitFile("src/c.txt", "c1\n", "feat: c")
	r.RunGit("merge", "--no-ff", "-m", "Merge branch 'feature/x' of origin into feature/x", "origin/feature/x")
	r.RunGit("push", "-q", "origin", "feature/x")
	r.Checkout("main", false)

	app, _ := testApp(t, r, testutil.NewGHMock())
	if err := app.Dev("feature/x"); err != nil {
		t.Fatalf("Dev: %v", err)
	}
	for _, f := range []string{"a", "b", "c"} {
		envHasFile(t, r, "dev", "src/"+f+".txt", f+"1")
	}
}

func TestDevRejectsMergeWithOwnChanges(t *testing.T) {
	r := testutil.NewRepo(t)

	// merge de outra feature com trabalho novo dentro do próprio merge commit.
	r.Checkout("feature/x", true)
	r.CommitFile("src/a.txt", "a1\n", "feat: a")
	r.Checkout("side", true)
	r.CommitFile("src/s.txt", "s1\n", "feat: side")
	r.Checkout("feature/x", false)
	r.RunGit("merge", "--no-ff", "--no-commit", "side")
	r.WriteFile("src/extra.txt", "e\n")
	r.RunGit("add", "-A")
	r.RunGit("commit", "-q", "-m", "feat: extra")
	r.Push("feature/x")
	r.Checkout("main", false)

	app, _ := testApp(t, r, testutil.NewGHMock())
	err := app.Dev("feature/x")
	if err == nil {
		t.Fatal("esperava recusa do merge com alterações próprias")
	}
	for _, want := range []string{"feat: extra", "alterações próprias", "git reset --soft origin/main"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("erro sem %q: %v", want, err)
		}
	}
}

func TestDevAcceptsMergeWithOwnChangesAlreadyInEnvs(t *testing.T) {
	r := testutil.NewRepo(t)

	// fluxo antigo: a feature, com o merge de trabalho próprio, foi mergeada
	// em dev e hom; depois ganhou mais um commit.
	r.Checkout("feature/x", true)
	r.CommitFile("src/a.txt", "a1\n", "feat: a")
	r.Checkout("side", true)
	r.CommitFile("src/s.txt", "s1\n", "feat: side")
	r.Checkout("feature/x", false)
	r.RunGit("merge", "--no-ff", "--no-commit", "side")
	r.WriteFile("src/extra.txt", "e\n")
	r.RunGit("add", "-A")
	r.RunGit("commit", "-q", "-m", "feat: extra")
	for _, env := range []string{"dev", "hom"} {
		r.Checkout(env, false)
		r.RunGit("merge", "--no-ff", "-m", "merge feature/x", "feature/x")
		r.Push(env)
	}
	r.Checkout("feature/x", false)
	r.CommitFile("src/b.txt", "b1\n", "feat: b")
	r.Push("feature/x")
	r.Checkout("main", false)

	app, _ := testApp(t, r, testutil.NewGHMock())
	if err := app.Dev("feature/x"); err != nil {
		t.Fatalf("Dev: %v", err)
	}
	envHasFile(t, r, "dev", "src/extra.txt", "e")
	envHasFile(t, r, "dev", "src/b.txt", "b1")
}

func TestDevRejectsMergeOfEnv(t *testing.T) {
	r := testutil.NewRepo(t)

	// dev tem um commit próprio que a feature traz por merge.
	r.Checkout("dev", false)
	r.CommitFile("src/dev-only.txt", "d\n", "chore: dev only")
	r.Push("dev")
	r.Checkout("main", false)
	r.Checkout("feature/x", true)
	r.CommitFile("src/a.txt", "a1\n", "feat: a")
	r.RunGit("merge", "--no-ff", "-m", "Merge branch 'dev' into feature/x", "dev")
	r.Push("feature/x")
	r.Checkout("main", false)

	app, _ := testApp(t, r, testutil.NewGHMock())
	err := app.Dev("feature/x")
	if err == nil || !strings.Contains(err.Error(), "Merge branch 'dev' into feature/x (merge de ambiente)") {
		t.Fatalf("esperava recusa listando o merge de dev, veio: %v", err)
	}
}

func TestDevAcceptsMergeOfEnvMergeAlreadyInProduction(t *testing.T) {
	r := testutil.NewRepo(t)

	// feature/y entra em hom por merge de PR e depois em main; feature/x faz
	// merge do merge de hom, que não traz nada além de produção.
	r.Checkout("feature/y", true)
	r.CommitFile("src/y.txt", "y\n", "feat: y")
	r.Push("feature/y")
	r.Checkout("hom", false)
	r.RunGit("merge", "--no-ff", "-m", "Merge pull request #7 from feature/y", "feature/y")
	r.Push("hom")
	r.Checkout("main", false)
	r.RunGit("merge", "--no-ff", "-m", "Merge pull request #9 from feature/y", "feature/y")
	r.Push("main")
	r.Checkout("feature/x", true)
	r.RunGit("reset", "-q", "--hard", "main~1")
	r.CommitFile("src/a.txt", "a1\n", "feat: a")
	r.RunGit("merge", "--no-ff", "-m", "merge hom", "hom")
	r.Push("feature/x")
	r.Checkout("main", false)

	app, _ := testApp(t, r, testutil.NewGHMock())
	if err := app.Dev("feature/x"); err != nil {
		t.Fatalf("Dev: %v", err)
	}
	envHasFile(t, r, "dev", "src/a.txt", "a1")
}

func TestDevRejectsBranchCreatedFromEnv(t *testing.T) {
	r := testutil.NewRepo(t)
	gh := testutil.NewGHMock()
	featureBranch(r, [2]string{"src/a.txt", "a1\n"})
	openPR(gh, 2, "feature/y")
	app, _ := testApp(t, r, gh)
	if err := app.Dev("feature/x"); err != nil {
		t.Fatalf("Dev: %v", err)
	}

	// feature/y nasce de origin/dev, que tem o cherry-pick de feature/x.
	r.Fetch()
	r.RunGit("checkout", "-b", "feature/y", "origin/dev")
	r.CommitFile("src/y.txt", "y\n", "feat: y")
	r.Push("feature/y")
	r.Checkout("main", false)

	err := app.Hom("feature/y")
	if err == nil {
		t.Fatal("esperava recusa de branch criada a partir de dev")
	}
	for _, want := range []string{"feat: src/a.txt (cherry-pick)", "git rebase -i origin/main"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("erro sem %q: %v", want, err)
		}
	}
}

func TestDevSkipsCommitsSquashedIntoEnv(t *testing.T) {
	r := testutil.NewRepo(t)
	featureBranch(r,
		[2]string{"src/a.txt", "a1\n"},
		[2]string{"src/b.txt", "b1\n"},
	)
	commits, err := r.G.Log("origin/main..origin/feature/x", true)
	if err != nil {
		t.Fatal(err)
	}

	// Fluxo antigo: o primeiro commit entrou em dev num squash do PR #7, junto
	// com um ajuste próprio de dev (patch-id diferente do commit original).
	r.Checkout("dev", false)
	r.WriteFile("src/a.txt", "a1\n")
	r.WriteFile("src/dev-only.txt", "d\n")
	r.RunGit("add", "-A")
	r.RunGit("commit", "-q", "-m", "feat: a [dev] (#7)")
	r.Push("dev")
	r.Checkout("main", false)

	gh := testutil.NewGHMock()
	gh.Commits[7] = []github.Commit{{AuthoredDate: commits[0].AuthorDate, Headline: commits[0].Subject()}}
	app, _ := testApp(t, r, gh)
	if err := app.Dev("feature/x"); err != nil {
		t.Fatalf("Dev: %v", err)
	}
	if got := pickedCount(t, r, "dev"); got != 1 {
		t.Fatalf("commits aplicados em dev = %d, queria só o que não veio no squash", got)
	}
	envHasFile(t, r, "dev", "src/b.txt", "b1")
}

func TestDevSkipsCommitsCopiedIntoEnv(t *testing.T) {
	r := testutil.NewRepo(t)
	featureBranch(r,
		[2]string{"src/a.txt", "a1\n"},
		[2]string{"src/b.txt", "b1\n"},
	)
	first := r.SHA("origin/feature/x~1")

	// Fluxo antigo: cópia do primeiro commit numa branch feature/x-dev, ajustada
	// na resolução de conflito (patch-id diferente) e mergeada em dev.
	r.Checkout("dev", false)
	r.Checkout("feature/x-dev", true)
	r.RunGit("cherry-pick", "--no-commit", first)
	r.WriteFile("src/dev-only.txt", "d\n")
	r.RunGit("add", "-A")
	r.RunGit("commit", "-q", "-C", first)
	r.Checkout("dev", false)
	r.RunGit("merge", "-q", "--no-ff", "-m", "Merge feature/x-dev", "feature/x-dev")
	r.Push("dev")
	r.Checkout("main", false)

	app, _ := testApp(t, r, testutil.NewGHMock())
	if err := app.Dev("feature/x"); err != nil {
		t.Fatalf("Dev: %v", err)
	}
	if got := pickedCount(t, r, "dev"); got != 1 {
		t.Fatalf("commits aplicados em dev = %d, queria só o que não tinha cópia", got)
	}
	envHasFile(t, r, "dev", "src/b.txt", "b1")
}
