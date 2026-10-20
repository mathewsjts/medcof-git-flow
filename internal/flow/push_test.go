package flow

import (
	"path/filepath"
	"testing"

	"github.com/mathewsjts/medcof-git-flow/internal/git"
	"github.com/mathewsjts/medcof-git-flow/internal/testutil"
)

// TestPushWithRetryRebasesOnConcurrency simula um push rejeitado por concorrência:
// origin/dev avança depois que o worktree foi criado; o push deve rebasear e retentar.
func TestPushWithRetryRebasesOnConcurrency(t *testing.T) {
	r := testutil.NewRepo(t)
	r.Fetch()

	// Worktree baseado em origin/dev (B0) com um commit nosso (toca X).
	common, err := r.G.CommonDir()
	if err != nil {
		t.Fatal(err)
	}
	wtPath := filepath.Join(common, "flow", "apply-dev-1")
	workBranch := "flow-apply-dev-1"
	if err := r.G.WorktreeAdd(wtPath, workBranch, "origin/dev"); err != nil {
		t.Fatalf("worktree add: %v", err)
	}
	defer r.G.WorktreeRemove(wtPath)
	wt := git.New(wtPath)
	r.WriteFileAt(wtPath, "x.txt", "x1\n")
	if err := wt.RunOK("add", "-A"); err != nil {
		t.Fatal(err)
	}
	if err := wt.Commit("feat: x"); err != nil {
		t.Fatal(err)
	}

	// Concorrência: outro ator empurra Y para origin/dev.
	r.Checkout("dev", false)
	r.CommitFile("y.txt", "y1\n", "feat: y")
	r.Push("dev")
	r.Checkout("main", false)

	env := Env{Logical: "dev", Branch: "dev"}
	conf, err := pushWithRetry(wt, workBranch, "dev", env, 1)
	if err != nil {
		t.Fatalf("pushWithRetry: %v", err)
	}
	if conf != nil {
		t.Fatalf("esperava sucesso, veio conflito: %+v", conf)
	}

	// origin/dev deve conter X (nosso, rebaseado) e Y (concorrente).
	if _, ok := r.FileOnOrigin("dev", "x.txt"); !ok {
		t.Error("x.txt ausente em origin/dev após retry")
	}
	if _, ok := r.FileOnOrigin("dev", "y.txt"); !ok {
		t.Error("y.txt ausente em origin/dev após retry")
	}
}
