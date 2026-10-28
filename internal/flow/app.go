// Package flow contém os casos de uso do CLI (setup, dev, hom, status),
// orquestrando os adaptadores git e gh.
package flow

import (
	"fmt"
	"io"

	"github.com/mathewsjts/medcof-git-flow/internal/config"
	"github.com/mathewsjts/medcof-git-flow/internal/git"
	"github.com/mathewsjts/medcof-git-flow/internal/github"
	"github.com/mathewsjts/medcof-git-flow/internal/ui"
)

// Versão mínima do git exigida (git show --remerge-diff).
const (
	MinGitMajor = 2
	MinGitMinor = 36
)

// App agrupa as dependências de um comando já resolvido para um repo gerenciado.
type App struct {
	Git  *git.Git
	GH   github.Client
	Cfg  *config.Config
	Repo config.Repo
	UI   *ui.Printer
}

// Deps permite injetar adaptadores (usado nos testes). Campos nil usam o padrão real.
type Deps struct {
	Git *git.Git
	GH  github.Client
	Out io.Writer
}

// CheckPrereqs valida a versão mínima do git e autenticação do gh.
func CheckPrereqs(g *git.Git, gh github.Client) error {
	ok, err := g.AtLeast(MinGitMajor, MinGitMinor)
	if err != nil {
		return fmt.Errorf("não foi possível verificar a versão do git: %w", err)
	}
	if !ok {
		return fmt.Errorf("git %d.%d ou superior é necessário (por causa de `git show --remerge-diff`)", MinGitMajor, MinGitMinor)
	}
	if err := gh.AuthStatus(); err != nil {
		return err
	}
	return nil
}

// Bootstrap resolve o repo gerenciado a partir do origin, checa pré-requisitos
// e devolve um App pronto. Recusa repositórios fora da lista embutida.
func Bootstrap(dir string, d Deps) (*App, error) {
	g := d.Git
	if g == nil {
		g = git.New(dir)
	}
	gh := d.GH
	if gh == nil {
		gh = github.NewCLI(dir)
	}
	out := d.Out
	if out == nil {
		return nil, fmt.Errorf("writer de saída não configurado")
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	if err := CheckPrereqs(g, gh); err != nil {
		return nil, err
	}

	remote, err := g.RemoteURL()
	if err != nil {
		return nil, fmt.Errorf("não foi possível ler o remote origin: %w", err)
	}
	name, err := config.NormalizeRemote(remote)
	if err != nil {
		return nil, err
	}
	repo, ok := cfg.Lookup(name)
	if !ok {
		return nil, fmt.Errorf("repositório %q não está na lista do flow; este CLI é exclusivo do time", name)
	}

	return &App{Git: g, GH: gh, Cfg: cfg, Repo: repo, UI: ui.New(out)}, nil
}
