// Package github é um adaptador sobre o `gh` CLI, atrás de uma interface para
// permitir mock nos testes. Nenhuma chamada usa shell.
package github

// PR representa os campos de um pull request que o flow consome.
type PR struct {
	Number         int
	Title          string
	State          string // OPEN, CLOSED, MERGED
	BaseRefName    string
	HeadRefName    string
	ReviewDecision string // APPROVED, CHANGES_REQUESTED, REVIEW_REQUIRED, ""
	Author         string // login do autor
	Merged         bool
	URL            string
	// Commits em ordem cronológica (mais antigo -> mais novo).
	Commits []Commit
}

// Commit é um commit de um PR (SHA + resumo).
type Commit struct {
	SHA      string
	Headline string
}

// CommitPR é a referência de um PR associado a um commit (via API REST).
type CommitPR struct {
	Number int
	State  string // open, closed
	Merged bool
	Author string
}

// Client é a interface sobre o `gh` usada pelos casos de uso.
type Client interface {
	// AuthStatus falha se o gh não estiver autenticado.
	AuthStatus() error
	// CurrentPR devolve o PR da branch corrente (gh pr view).
	CurrentPR() (PR, error)
	// PR devolve um PR pelo número.
	PR(number int) (PR, error)
	// CommitPRs devolve os PRs associados a um commit (owner/repo, sha).
	CommitPRs(repo, sha string) ([]CommitPR, error)
	// CommitAuthor devolve o login (ou nome) do autor de um commit.
	CommitAuthor(repo, sha string) (string, error)
}
