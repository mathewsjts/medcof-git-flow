package flow

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mathewsjts/medcof-git-flow/internal/git"
	"github.com/mathewsjts/medcof-git-flow/internal/github"
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
// novo. Merge commits são pulados: os commits que eles trazem já estão na
// lista, seja de produção (que não entra), da própria feature (`git pull` sem
// rebase) ou de outra feature (dependência). Merges de produção são pulados
// mesmo com resolução de conflito, que fica de fora; os demais merges com
// alterações próprias são recusados, porque elas se perderiam, a não ser que o
// merge já esteja em todos os ambientes. Também recusa
// merges que trazem commits de um ambiente e commits com "(cherry picked from
// commit X)": são sinal de branch criada a partir de dev/hom (ou que fez merge
// deles), e levariam a história própria do ambiente para os outros ambientes e
// para main.
func (a *App) featureCommits(ref string) ([]git.LogCommit, error) {
	prod := "origin/" + a.Repo.Production
	all, err := a.Git.Log(prod+".."+ref, true)
	if err != nil {
		return nil, err
	}
	var commits []git.LogCommit
	var fromEnv, ownChanges []string
	for _, c := range all {
		line := fmt.Sprintf("  %s %s", c.Short(), c.Subject())
		if len(c.Parents) > 1 {
			kind, err := a.classifyMerge(c, prod)
			if err != nil {
				return nil, err
			}
			switch kind {
			case mergeOwnChanges:
				ownChanges = append(ownChanges, line)
			case mergeFromEnv:
				fromEnv = append(fromEnv, line+" (merge de ambiente)")
			}
			continue
		}
		if cherryFrom(c) != "" {
			fromEnv = append(fromEnv, line+" (cherry-pick)")
			continue
		}
		commits = append(commits, c)
	}
	var msgs []string
	if len(fromEnv) > 0 {
		msgs = append(msgs, fmt.Sprintf("a branch tem commits que parecem vir de um ambiente (branch criada a partir de dev/hom ou merge deles):\n%s\nrefaça a base em origin/%s mantendo só os seus commits (ex.: `git rebase -i origin/%s`)",
			strings.Join(fromEnv, "\n"), a.Repo.Production, a.Repo.Production))
	}
	if len(ownChanges) > 0 {
		msgs = append(msgs, fmt.Sprintf("a branch tem merge commits com alterações próprias, que o flow não aplica (só aplica commits comuns):\n%s\njunte a branch num commit comum sem perder essas alterações (rebase descarta merges): `git merge origin/%s && git reset --soft origin/%s && git commit`, e `git push --force-with-lease`",
			strings.Join(ownChanges, "\n"), a.Repo.Production, a.Repo.Production))
	}
	if len(msgs) > 0 {
		return nil, errors.New(strings.Join(msgs, "\n\n"))
	}
	return commits, nil
}

type mergeKind int

const (
	mergeClean      mergeKind = iota // pulado sem perda
	mergeOwnChanges                  // tem alterações além das que traz
	mergeFromEnv                     // traz commits de um ambiente
)

// classifyMerge decide o que fazer com o merge c. São limpos os merges que só
// trazem produção (mesmo com resolução de conflito) e os que já estão em todos
// os ambientes (fluxo antigo): não há o que levar a nenhum deles. Dos demais,
// conta como alteração própria tudo que o merge muda além do merge automático
// dos parents (resolução de conflito inclusa), e como vindo de um ambiente os
// commits comuns que ele traz de dev/hom: um merge de PR no ambiente cujos
// lados já estão em produção não leva nada do ambiente adiante.
func (a *App) classifyMerge(c git.LogCommit, prod string) (mergeKind, error) {
	exclude := []string{"^" + c.Parents[0], "^" + prod}
	onlyProd := true
	for _, p := range c.Parents[1:] {
		n, err := a.Git.Count(append([]string{p}, exclude...)...)
		if err != nil {
			return 0, err
		}
		onlyProd = onlyProd && n == 0
	}
	if onlyProd {
		return mergeClean, nil
	}
	var envs []string
	inAllEnvs := true
	for _, e := range a.Repo.Envs() {
		b, _ := a.Repo.Branch(e)
		envs = append(envs, "^origin/"+b)
		inAllEnvs = inAllEnvs && a.Git.IsAncestor(c.SHA, "origin/"+b)
	}
	if inAllEnvs {
		return mergeClean, nil
	}
	own, err := a.Git.MergeHasOwnChanges(c.SHA)
	if err != nil {
		return 0, err
	}
	if own {
		return mergeOwnChanges, nil
	}
	exclude = append(exclude, "--no-merges")
	for _, p := range c.Parents[1:] {
		side, err := a.Git.Count(append([]string{p}, exclude...)...)
		if err != nil {
			return 0, err
		}
		notInEnv, err := a.Git.Count(append(append([]string{p}, exclude...), envs...)...)
		if err != nil {
			return 0, err
		}
		if notInEnv < side {
			return mergeFromEnv, nil
		}
	}
	return mergeClean, nil
}

// missing separa os commits da feature entre os que faltam no ambiente e os que
// já estão lá: por ancestralidade (mergeados no fluxo antigo), por patch-id
// (git cherry), pela linha "(cherry picked from commit X)" de uma aplicação
// anterior, inclusive as que passaram por resolução de conflito, por uma cópia
// com a mesma data de autoria e título (rebase ou branch -<ambiente> do fluxo
// antigo) ou dentro de um squash merge de PR no ambiente (fluxo antigo).
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
	envCommits, err := a.envCommits(envBranch)
	if err != nil {
		return nil, nil, err
	}
	picked := pickedInto(envCommits)
	copied := copiedInto(envCommits)
	var candidates []git.LogCommit
	for _, c := range commits {
		if !outside[c.SHA] || patchApplied[c.SHA] || picked(c.SHA) || copied[copyKey(c)] {
			applied = append(applied, c)
		} else {
			candidates = append(candidates, c)
		}
	}
	squashed := a.squashedInto(envCommits, candidates)
	for _, c := range candidates {
		if squashed(c) {
			applied = append(applied, c)
		} else {
			pending = append(pending, c)
		}
	}
	return pending, applied, nil
}

// envCommits devolve os commits do ambiente desde o merge-base com produção.
func (a *App) envCommits(envBranch string) ([]git.LogCommit, error) {
	envRef := "origin/" + envBranch
	mb, err := a.Git.MergeBase("origin/"+a.Repo.Production, envRef)
	if err != nil {
		return nil, err
	}
	return a.Git.Log(mb+".."+envRef, false)
}

// copyKey identifica um commit pela data de autoria e título, que sobrevivem às
// cópias por cherry-pick e rebase.
func copyKey(c git.LogCommit) string {
	return c.AuthorDate.UTC().Format(time.RFC3339) + "|" + c.Subject()
}

// copiedInto indexa os commits do ambiente por copyKey.
func copiedInto(envCommits []git.LogCommit) map[string]bool {
	keys := map[string]bool{}
	for _, c := range envCommits {
		keys[copyKey(c)] = true
	}
	return keys
}

var reSquashPR = regexp.MustCompile(`\(#(\d+)\)$`)

// squashedInto devolve um teste "este commit entrou no ambiente dentro de um
// squash merge": consulta no GitHub os PRs dos squashes "... (#N)" do ambiente
// feitos depois do commit mais antigo e casa os commits deles por data de
// autoria e título (o GitHub trunca títulos longos com "…").
func (a *App) squashedInto(envCommits, candidates []git.LogCommit) func(git.LogCommit) bool {
	none := func(git.LogCommit) bool { return false }
	if len(candidates) == 0 {
		return none
	}
	since := candidates[0].AuthorDate
	for _, c := range candidates {
		if c.AuthorDate.Before(since) {
			since = c.AuthorDate
		}
	}
	var prCommits []github.Commit
	for _, c := range envCommits {
		m := reSquashPR.FindStringSubmatch(c.Subject())
		if len(c.Parents) != 1 || m == nil || c.AuthorDate.Before(since) {
			continue
		}
		n, _ := strconv.Atoi(m[1])
		cs, err := a.GH.PRCommits(n)
		if err != nil {
			a.UI.Warn("não consegui ler os commits do PR #%d para checar squash: %v", n, err)
			continue
		}
		prCommits = append(prCommits, cs...)
	}
	if len(prCommits) == 0 {
		return none
	}
	return func(c git.LogCommit) bool {
		for _, pc := range prCommits {
			if pc.AuthoredDate.Equal(c.AuthorDate) && strings.HasPrefix(c.Subject(), strings.TrimSuffix(pc.Headline, "…")) {
				return true
			}
		}
		return false
	}
}

// pickedInto devolve um teste "este SHA já foi aplicado por cherry-pick -x no
// ambiente".
func pickedInto(envCommits []git.LogCommit) func(sha string) bool {
	var from []string
	for _, c := range envCommits {
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
	}
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
