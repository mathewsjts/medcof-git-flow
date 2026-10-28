package flow

import (
	"strings"
	"testing"

	"github.com/mathewsjts/medcof-git-flow/internal/testutil"
)

func TestStatusPerEnv(t *testing.T) {
	r := testutil.NewRepo(t)
	gh := testutil.NewGHMock()
	featureBranch(r, [2]string{"src/a.txt", "a1\n"})
	openPR(gh, 8, "feature/x")
	app, out := testApp(t, r, gh)
	if err := app.Dev("8"); err != nil {
		t.Fatalf("Dev: %v", err)
	}

	r.Checkout("feature/x", false)
	r.CommitFile("src/b.txt", "b1\n", "feat: b")
	r.Push("feature/x")

	out.Reset()
	if err := app.Status(""); err != nil {
		t.Fatalf("Status: %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"PR #8 (feature/x)",
		"aprovado",
		"dev: parcial (1/2); faltam:",
		"feat: b",
		"hom: ausente (0/2)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("saída sem %q:\n%s", want, got)
		}
	}

	if err := app.Dev("8"); err != nil {
		t.Fatalf("Dev: %v", err)
	}
	out.Reset()
	if err := app.Status("feature/x"); err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !strings.Contains(out.String(), "dev: aplicado (2/2)") {
		t.Errorf("esperava dev aplicado:\n%s", out)
	}
}

func TestStatusBranchWithoutPR(t *testing.T) {
	r := testutil.NewRepo(t)
	featureBranch(r, [2]string{"src/a.txt", "a1\n"})
	app, out := testApp(t, r, testutil.NewGHMock())

	if err := app.Status("feature/x"); err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !strings.Contains(out.String(), "sem PR aberto para main") {
		t.Errorf("esperava indicação de branch sem PR:\n%s", out)
	}
}
