package testutil

import "testing"

func TestHarnessBasics(t *testing.T) {
	r := NewRepo(t)

	// Branches de ambiente existem no origin.
	for _, b := range []string{"main", "dev", "hom"} {
		if _, err := r.G.Run("rev-parse", "--verify", "origin/"+b); err != nil {
			t.Fatalf("branch origin/%s ausente: %v", b, err)
		}
	}

	// Cria feature a partir de main e empurra.
	r.Checkout("feature/x", true)
	c1 := r.CommitFile("src/a.txt", "a1\n", "feat: a")
	c2 := r.CommitFile("src/b.txt", "b1\n", "feat: b")
	r.Push("feature/x")

	if r.SHA("HEAD") != c2 {
		t.Fatalf("HEAD = %s, queria %s", r.SHA("HEAD"), c2)
	}

	// merge-base(main, feature) == ponta de main.
	mb, err := r.G.MergeBase("origin/main", "feature/x")
	if err != nil {
		t.Fatal(err)
	}
	if mb != r.SHA("origin/main") {
		t.Fatalf("merge-base = %s, queria %s", mb, r.SHA("origin/main"))
	}

	// rev-list main..feature (reverse) = [c1, c2].
	shas, err := r.G.RevListRange("origin/main..feature/x", true, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(shas) != 2 || shas[0] != c1 || shas[1] != c2 {
		t.Fatalf("rev-list = %v, queria [%s %s]", shas, c1, c2)
	}
}
