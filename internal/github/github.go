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
	URL            string
}

// Client é a interface sobre o `gh` usada pelos casos de uso.
type Client interface {
	// AuthStatus falha se o gh não estiver autenticado.
	AuthStatus() error
	// PR devolve um PR pelo número.
	PR(number int) (PR, error)
	// PRForBranch devolve o PR aberto com head em branch e base em base;
	// found=false quando não há.
	PRForBranch(branch, base string) (pr PR, found bool, err error)
}
