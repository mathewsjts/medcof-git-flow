package flow

import (
	"github.com/mathewsjts/medcof-git-flow/internal/git"
	"github.com/mathewsjts/medcof-git-flow/internal/github"
	"github.com/mathewsjts/medcof-git-flow/internal/ui"
)

// Setup checa pré-requisitos (git, gh) e ativa rerere.enabled no repo local.
// Não exige que o repo esteja na lista gerenciada: é uma checagem de ambiente.
func Setup(g *git.Git, gh github.Client, p *ui.Printer) error {
	ma, mi, err := g.Version()
	if err != nil {
		p.Fail("não foi possível executar o git: %v", err)
		return err
	}
	if ok, _ := g.AtLeast(MinGitMajor, MinGitMinor); !ok {
		p.Fail("git %d.%d encontrado; é necessário %d.%d ou superior", ma, mi, MinGitMajor, MinGitMinor)
		return errGitTooOld
	}
	p.OK("git %d.%d (>= %d.%d)", ma, mi, MinGitMajor, MinGitMinor)

	if err := gh.AuthStatus(); err != nil {
		p.Fail("gh não autenticado: rode `gh auth login`")
		return err
	}
	p.OK("gh autenticado")

	if err := g.SetConfig("rerere.enabled", "true"); err != nil {
		p.Fail("não foi possível ativar rerere.enabled: %v", err)
		return err
	}
	p.OK("rerere.enabled ativado no repositório local")

	p.Info("Tudo pronto. Use `flow dev` na sua branch de feature.")
	return nil
}
