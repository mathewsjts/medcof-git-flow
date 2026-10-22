package flow

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mathewsjts/medcof-git-flow/internal/git"
)

// Env é um ambiente lógico resolvido para a branch real.
type Env struct {
	Logical string // "dev" / "hom"
	Branch  string // "develop" / "hom"
}

// String descreve o ambiente como "dev (develop)".
func (e Env) String() string {
	if e.Logical == e.Branch {
		return e.Logical
	}
	return fmt.Sprintf("%s (%s)", e.Logical, e.Branch)
}

// Env resolve um ambiente lógico a partir da config do repo.
func (a *App) Env(logical string) (Env, error) {
	b, ok := a.Repo.Branch(logical)
	if !ok {
		return Env{}, fmt.Errorf("ambiente %q não existe para %s", logical, a.Repo.Name)
	}
	return Env{Logical: logical, Branch: b}, nil
}

// featureCommits devolve os commits de produção..ref, do mais antigo ao mais
// novo. Recusa merge commits e commits com "(cherry picked from commit X)":
// são sinal de branch criada a partir de dev/hom (ou que fez merge deles), e
// levariam a história própria do ambiente para os outros ambientes e para main.
func (a *App) featureCommits(ref string) ([]git.LogCommit, error) {
	commits, err := a.Git.Log("origin/"+a.Repo.Production+".."+ref, true)
	if err != nil {
		return nil, err
	}
	var bad []string
	for _, c := range commits {
		switch {
		case len(c.Parents) > 1:
			bad = append(bad, fmt.Sprintf("  %s %s (merge commit)", c.Short(), c.Subject()))
		case cherryFrom(c) != "":
			bad = append(bad, fmt.Sprintf("  %s %s (cherry-pick)", c.Short(), c.Subject()))
		}
	}
	if len(bad) > 0 {
		return nil, fmt.Errorf("a branch tem commits que parecem vir de um ambiente (branch criada a partir de dev/hom ou merge deles):\n%s\nrefaça a base em origin/%s mantendo só os seus commits (ex.: `git rebase -i origin/%s`)",
			strings.Join(bad, "\n"), a.Repo.Production, a.Repo.Production)
	}
	return commits, nil
}

// missing separa os commits da feature entre os que faltam no ambiente e os que
// já estão lá: por ancestralidade (mergeados no fluxo antigo), por patch-id
// (git cherry) ou pela linha "(cherry picked from commit X)" de uma aplicação
// anterior, inclusive as que passaram por resolução de conflito.
func (a *App) missing(commits []git.LogCommit, ref, envBranch string) (pending, applied []git.LogCommit, err error) {
	envRef := "origin/" + envBranch
	notInEnv, err := a.Git.RevListRange(envRef+".."+ref, false, false)
	if err != nil {
		return nil, nil, err
	}
	outside := map[string]bool{}
	for _, sha := range notInEnv {
		outside[sha] = true
	}
	patchApplied, err := a.Git.CherryEquivalent(envRef, ref, "origin/"+a.Repo.Production)
	if err != nil {
		return nil, nil, err
	}
	picked, err := a.pickedInto(envBranch)
	if err != nil {
		return nil, nil, err
	}
	for _, c := range commits {
		if !outside[c.SHA] || patchApplied[c.SHA] || picked(c.SHA) {
			applied = append(applied, c)
		} else {
			pending = append(pending, c)
		}
	}
	return pending, applied, nil
}

// pickedInto devolve um teste "este SHA já foi aplicado por cherry-pick -x no
// ambiente", olhando os commits do ambiente desde o merge-base com produção.
func (a *App) pickedInto(envBranch string) (func(sha string) bool, error) {
	envRef := "origin/" + envBranch
	mb, err := a.Git.MergeBase("origin/"+a.Repo.Production, envRef)
	if err != nil {
		return nil, err
	}
	commits, err := a.Git.Log(mb+".."+envRef, false)
	if err != nil {
		return nil, err
	}
	var from []string
	for _, c := range commits {
		if f := cherryFrom(c); f != "" {
			from = append(from, f)
		}
	}
	return func(sha string) bool {
		for _, f := range from {
			if matchesSHA(f, sha) {
				return true
			}
		}
		return false
	}, nil
}

// applyInWorktree aplica os commits num worktree temporário destacado em
// origin/<env> e faz push, sem tocar no working tree do usuário. Num conflito,
// aborta sem enviar nada e devolve o commit que conflitou.
func (a *App) applyInWorktree(env Env, pending []git.LogCommit) (*git.LogCommit, error) {
	common, err := a.Git.CommonDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(common, "flow", "apply-"+env.Logical)
	a.Git.WorktreeRemove(path)
	if err := a.Git.WorktreeAddDetached(path, "origin/"+env.Branch); err != nil {
		return nil, fmt.Errorf("não foi possível criar worktree: %w", err)
	}
	defer a.Git.WorktreeRemove(path)
	wt := a.Git.At(path)

	for i := range pending {
		if err := wt.CherryPick(pending[i].SHA); err != nil {
			wt.CherryPickAbort()
			return &pending[i], nil
		}
	}
	return nil, pushWithRetry(wt, env.Branch)
}

// pushWithRetry faz push do HEAD do worktree para envBranch com até 3
// tentativas, rebaseando sobre origin/<envBranch> quando alguém enviou antes.
func pushWithRetry(wt *git.Git, envBranch string) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if lastErr = wt.Push("HEAD:" + envBranch); lastErr == nil {
			return nil
		}
		if err := wt.Fetch(envBranch); err != nil {
			return fmt.Errorf("falha no fetch durante retry de push: %w", err)
		}
		if err := wt.Rebase("origin/" + envBranch); err != nil {
			wt.RebaseAbort()
			return fmt.Errorf("origin/%s mudou durante a aplicação e conflita com ela; nada foi enviado, rode o comando de novo", envBranch)
		}
	}
	return fmt.Errorf("push para %s rejeitado após 3 tentativas: %w", envBranch, lastErr)
}

// pushResolution envia a branch local do ambiente quando ela está à frente de
// origin só com cherry-picks desta feature: é a resolução de um conflito aberto
// por uma execução anterior. Devolve quantos commits foram enviados.
func (a *App) pushResolution(env Env, commits []git.LogCommit) (int, error) {
	if !a.Git.BranchExists(env.Branch) {
		return 0, nil
	}
	ahead, err := a.Git.Log("origin/"+env.Branch+".."+env.Branch, false)
	if err != nil || len(ahead) == 0 {
		return 0, err
	}
	for _, c := range ahead {
		if !containsSHA(commits, cherryFrom(c)) {
			return 0, nil
		}
	}
	if err := a.Git.Push(env.Branch + ":" + env.Branch); err != nil {
		return 0, fmt.Errorf("não foi possível enviar sua resolução em %s (origin/%s pode ter avançado; em %s rode `git pull --rebase` e o comando de novo): %w", env.Branch, env.Branch, env.Branch, err)
	}
	return len(ahead), a.Git.Fetch(env.Branch)
}
