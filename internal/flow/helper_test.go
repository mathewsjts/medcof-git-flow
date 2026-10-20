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
func testApp(t *testing.T, r *testutil.Repo, gh *testutil.GHMock, shared bool, verify [][]string) (*App, *bytes.Buffer) {
	t.Helper()
	buf := &bytes.Buffer{}
	repo := config.Repo{
		Name:         "test/app",
		Environments: map[string]string{"dev": "dev", "hom": "hom"},
		Production:   "main",
		Shared:       shared,
		Verify:       verify,
	}
	cfg := &config.Config{Repos: map[string]config.Repo{repo.Name: repo}}
	app := &App{Git: r.G, GH: gh, Cfg: cfg, Repo: repo, UI: ui.New(buf)}
	return app, buf
}

// openPR registra um PR aberto com base main e head na branch dada.
func openPR(gh *testutil.GHMock, num int, head string, current bool) github.PR {
	pr := github.PR{Number: num, State: "OPEN", BaseRefName: "main", HeadRefName: head, ReviewDecision: "APPROVED"}
	gh.AddPR(pr, current)
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

// countTrailer conta quantos commits de origin/<branch> têm o trailer Flow-PR do PR.
func metasOnOrigin(t *testing.T, r *testutil.Repo, branch string) []CommitMeta {
	t.Helper()
	var metas []CommitMeta
	for _, c := range r.BodiesOnOrigin(branch) {
		metas = append(metas, parseMeta(c))
	}
	return metas
}
