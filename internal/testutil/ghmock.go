package testutil

import (
	"fmt"

	"github.com/mathewsjts/medcof-git-flow/internal/github"
)

// GHMock é um github.Client configurável para testes.
type GHMock struct {
	// PRs indexado por número.
	PRs map[int]github.PR
	// Current é o número do PR da "branch atual" (para CurrentPR).
	Current int
	// CommitPRsBySHA mapeia SHA -> PRs associados (caso B').
	CommitPRsBySHA map[string][]github.CommitPR
	// AuthorBySHA mapeia SHA -> autor (caso B' sem PR).
	AuthorBySHA map[string]string
	// AuthErr, se != nil, faz AuthStatus falhar.
	AuthErr error
}

// NewGHMock cria um mock vazio.
func NewGHMock() *GHMock {
	return &GHMock{
		PRs:            map[int]github.PR{},
		CommitPRsBySHA: map[string][]github.CommitPR{},
		AuthorBySHA:    map[string]string{},
	}
}

var _ github.Client = (*GHMock)(nil)

// AddPR registra um PR e opcionalmente o marca como o PR corrente.
func (m *GHMock) AddPR(pr github.PR, current bool) {
	m.PRs[pr.Number] = pr
	if current {
		m.Current = pr.Number
	}
}

func (m *GHMock) AuthStatus() error { return m.AuthErr }

func (m *GHMock) CurrentPR() (github.PR, error) {
	if m.Current == 0 {
		return github.PR{}, fmt.Errorf("nenhum PR encontrado para a branch atual")
	}
	return m.PR(m.Current)
}

func (m *GHMock) PR(number int) (github.PR, error) {
	pr, ok := m.PRs[number]
	if !ok {
		return github.PR{}, fmt.Errorf("PR #%d não encontrado", number)
	}
	return pr, nil
}

func (m *GHMock) CommitPRs(_, sha string) ([]github.CommitPR, error) {
	return m.CommitPRsBySHA[sha], nil
}

func (m *GHMock) CommitAuthor(_, sha string) (string, error) {
	if a, ok := m.AuthorBySHA[sha]; ok {
		return a, nil
	}
	return "", nil
}
