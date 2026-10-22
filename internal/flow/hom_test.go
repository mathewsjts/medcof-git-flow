package flow

import (
	"strings"
	"testing"

	"github.com/mathewsjts/medcof-git-flow/internal/testutil"
)

func TestHomAppliesApprovedPR(t *testing.T) {
	r := testutil.NewRepo(t)
	gh := testutil.NewGHMock()
	featureBranch(r, [2]string{"src/a.txt", "a1\n"})
	openPR(gh, 5, "feature/x")
	app, _ := testApp(t, r, gh)

	if err := app.Hom("feature/x"); err != nil {
		t.Fatalf("Hom: %v", err)
	}
	envHasFile(t, r, "hom", "src/a.txt", "a1")
	if _, ok := r.FileOnOrigin("dev", "src/a.txt"); ok {
		t.Fatal("hom não deve mexer em dev")
	}
}

func TestHomRejectsBranchWithoutPR(t *testing.T) {
	r := testutil.NewRepo(t)
	featureBranch(r, [2]string{"src/a.txt", "a1\n"})
	app, _ := testApp(t, r, testutil.NewGHMock())

	err := app.Hom("feature/x")
	if err == nil || !strings.Contains(err.Error(), "não tem PR aberto") {
		t.Fatalf("esperava recusa por falta de PR, veio: %v", err)
	}
}

func TestHomRejectsUnapprovedPR(t *testing.T) {
	r := testutil.NewRepo(t)
	gh := testutil.NewGHMock()
	featureBranch(r, [2]string{"src/a.txt", "a1\n"})
	pr := openPR(gh, 5, "feature/x")
	pr.ReviewDecision = "CHANGES_REQUESTED"
	gh.AddPR(pr)
	app, _ := testApp(t, r, gh)

	err := app.Hom("5")
	if err == nil || !strings.Contains(err.Error(), "mudanças pedidas") {
		t.Fatalf("esperava recusa por PR não aprovado, veio: %v", err)
	}
	if _, ok := r.FileOnOrigin("hom", "src/a.txt"); ok {
		t.Fatal("nada deveria ir para hom")
	}
}
