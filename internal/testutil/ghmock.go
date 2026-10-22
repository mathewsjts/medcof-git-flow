package testutil

import (
	"fmt"

	"github.com/mathewsjts/medcof-git-flow/internal/github"
)

// GHMock é um github.Client configurável para testes.
type GHMock struct {
	// PRs indexado por número.
	PRs map[int]github.PR
	// AuthErr, se != nil, faz AuthStatus falhar.
	AuthErr error
}

// NewGHMock cria um mock vazio.
func NewGHMock() *GHMock {
	return &GHMock{PRs: map[int]github.PR{}}
}

var _ github.Client = (*GHMock)(nil)

// AddPR registra um PR.
func (m *GHMock) AddPR(pr github.PR) {
	m.PRs[pr.Number] = pr
}

func (m *GHMock) AuthStatus() error { return m.AuthErr }

func (m *GHMock) PR(number int) (github.PR, error) {
	pr, ok := m.PRs[number]
	if !ok {
		return github.PR{}, fmt.Errorf("PR #%d não encontrado", number)
	}
	return pr, nil
}

func (m *GHMock) PRForBranch(branch, base string) (github.PR, bool, error) {
	for _, pr := range m.PRs {
		if pr.HeadRefName == branch && pr.BaseRefName == base && pr.State == "OPEN" {
			return pr, true, nil
		}
	}
	return github.PR{}, false, nil
}
