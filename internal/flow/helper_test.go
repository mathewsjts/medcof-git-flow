package flow

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mathewsjts/medcof-git-flow/internal/config"
	"github.com/mathewsjts/medcof-git-flow/internal/github"
	"github.com/mathewsjts/medcof-git-flow/internal/testutil"
	"github.com/mathewsjts/medcof-git-flow/internal/ui"
)

// testApp monta um App ligado ao clone de teste e a um GHMock, sem passar pelo
// Bootstrap (que exige repo gerenciado e pré-requisitos reais).
func testApp(t *testing.T, r *testutil.Repo, gh *testutil.GHMock) (*App, *bytes.Buffer) {
	t.Helper()
	buf := &bytes.Buffer{}
	repo := config.Repo{
		Name:         "test/app",
		Environments: map[string]string{"dev": "dev", "hom": "hom"},
		Production:   "main",
	}
	cfg := &config.Config{Repos: map[string]config.Repo{repo.Name: repo}}
	app := &App{Git: r.G, GH: gh, Cfg: cfg, Repo: repo, UI: ui.New(buf)}
	return app, buf
}

// openPR registra um PR aberto e aprovado com base main e head na branch dada.
func openPR(gh *testutil.GHMock, num int, head string) github.PR {
	pr := github.PR{Number: num, State: "OPEN", BaseRefName: "main", HeadRefName: head, ReviewDecision: "APPROVED"}
	gh.AddPR(pr)
	return pr
}

// envHasFile confirma que um arquivo com o conteúdo esperado existe em origin/<branch>.
func envHasFile(t *testing.T, r *testutil.Repo, branch, rel, want string) {
	t.Helper()
	got, ok := r.FileOnOrigin(branch, rel)
	if !ok {
		t.Fatalf("arquivo %s ausente em origin/%s", rel, branch)
	}
	if strings.TrimSpace(got) != strings.TrimSpace(want) {
		t.Fatalf("conteúdo de %s em %s = %q, queria %q", rel, branch, got, want)
	}
}

// pickedCount conta os commits de origin/<branch> aplicados por cherry-pick -x.
func pickedCount(t *testing.T, r *testutil.Repo, branch string) int {
	t.Helper()
	n := 0
	for _, c := range r.BodiesOnOrigin(branch) {
		if cherryFrom(c) != "" {
			n++
		}
	}
	return n
}

// currentBranch devolve a branch do working tree de teste.
func currentBranch(t *testing.T, r *testutil.Repo) string {
	t.Helper()
	b, err := r.G.CurrentBranch()
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// featureBranch cria feature/x a partir de main com um commit por arquivo,
// faz push e volta para main.
func featureBranch(r *testutil.Repo, files ...[2]string) {
	r.Checkout("feature/x", true)
	for _, f := range files {
		r.CommitFile(f[0], f[1], "feat: "+f[0])
	}
	r.Push("feature/x")
	r.Checkout("main", false)
}
