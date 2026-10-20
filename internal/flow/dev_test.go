package flow

import (
	"strings"
	"testing"

	"github.com/mathewsjts/medcof-git-flow/internal/testutil"
)

func TestDevInitialApply(t *testing.T) {
	r := testutil.NewRepo(t)
	gh := testutil.NewGHMock()

	r.Checkout("feature/x", true)
	r.CommitFile("src/a.txt", "a1\n", "feat: a")
	r.CommitFile("src/b.txt", "b1\n", "feat: b")
	r.Push("feature/x")
	r.Checkout("main", false)

	openPR(gh, 123, "feature/x", true)
	app, _ := testApp(t, r, gh, false, nil)

	if err := app.Dev(); err != nil {
		t.Fatalf("Dev: %v", err)
	}

	envHasFile(t, r, "dev", "src/a.txt", "a1")
	envHasFile(t, r, "dev", "src/b.txt", "b1")

	// Dois commits aplicados, cada um com trailer Flow-PR #123 e linha de procedência.
	metas := metasOnOrigin(t, r, "dev")
	applied := 0
	for _, m := range metas {
		if m.FlowPR == 123 {
			applied++
			if m.CherryFrom == "" {
				t.Errorf("commit %s sem linha 'cherry picked from'", m.SHA)
			}
		}
	}
	if applied != 2 {
		t.Fatalf("esperava 2 commits com Flow-PR #123, veio %d", applied)
	}
}

func TestDevReapplyOnlyNew(t *testing.T) {
	r := testutil.NewRepo(t)
	gh := testutil.NewGHMock()

	r.Checkout("feature/x", true)
	r.CommitFile("src/a.txt", "a1\n", "feat: a")
	r.CommitFile("src/b.txt", "b1\n", "feat: b")
	r.Push("feature/x")
	r.Checkout("main", false)
	openPR(gh, 7, "feature/x", true)
	app, _ := testApp(t, r, gh, false, nil)
	if err := app.Dev(); err != nil {
		t.Fatalf("primeira Dev: %v", err)
	}

	// Novo commit na feature após o review.
	r.Checkout("feature/x", false)
	r.CommitFile("src/c.txt", "c1\n", "feat: c")
	r.Push("feature/x")
	r.Checkout("main", false)

	if err := app.Dev(); err != nil {
		t.Fatalf("segunda Dev: %v", err)
	}
	envHasFile(t, r, "dev", "src/c.txt", "c1")

	// dev deve ter 3 commits com Flow-PR #7 (a, b, c), nenhum duplicado.
	count := 0
	for _, m := range metasOnOrigin(t, r, "dev") {
		if m.FlowPR == 7 {
			count++
		}
	}
	if count != 3 {
		t.Fatalf("esperava 3 commits Flow-PR #7 em dev, veio %d", count)
	}
}

func TestDevSkipsAlreadyAppliedAfterRebase(t *testing.T) {
	r := testutil.NewRepo(t)
	gh := testutil.NewGHMock()

	r.Checkout("feature/x", true)
	r.CommitFile("src/a.txt", "a1\n", "feat: a")
	r.Push("feature/x")
	r.Checkout("main", false)
	openPR(gh, 9, "feature/x", true)
	app, _ := testApp(t, r, gh, false, nil)
	if err := app.Dev(); err != nil {
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

	if err := app.Dev(); err != nil {
		t.Fatalf("Dev pós-rebase: %v", err)
	}
	envHasFile(t, r, "dev", "src/d.txt", "d1")

	// 'a' não deve ser reaplicado: dev tem exatamente 2 commits Flow-PR #9 (a + d).
	count := 0
	for _, m := range metasOnOrigin(t, r, "dev") {
		if m.FlowPR == 9 {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("esperava 2 commits Flow-PR #9 em dev (a+d), veio %d", count)
	}
}

func TestDevRejectsMergeCommits(t *testing.T) {
	r := testutil.NewRepo(t)
	gh := testutil.NewGHMock()

	// feature com um merge commit no intervalo.
	r.Checkout("feature/x", true)
	r.CommitFile("src/a.txt", "a1\n", "feat: a")
	// cria uma branch lateral e faz merge para introduzir merge commit
	r.Checkout("side", true)
	r.CommitFile("src/s.txt", "s1\n", "feat: side")
	r.Checkout("feature/x", false)
	r.RunGit("merge", "--no-ff", "-m", "merge side", "side")
	r.Push("feature/x")
	r.Checkout("main", false)

	openPR(gh, 11, "feature/x", true)
	app, _ := testApp(t, r, gh, false, nil)
	err := app.Dev()
	if err == nil || !strings.Contains(err.Error(), "merge commits") {
		t.Fatalf("esperava recusa por merge commits, veio: %v", err)
	}
}
