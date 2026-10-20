package flow

import (
	"fmt"
	"path/filepath"
	"strconv"
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

// Conflict descreve uma parada por conflito que exige diagnóstico/resolução.
type Conflict struct {
	Env       Env
	PR        int
	Phase     string   // "cherry-pick" ou "rebase"
	FailedSHA string   // commit da feature que falhou (vazio em rebase)
	Files     []string // arquivos em conflito
	Pending   []string // SHAs da feature ainda não aplicados, em ordem
}

// ApplyOutcome é o resultado de aplicar commits em um ambiente.
type ApplyOutcome struct {
	Applied  []string
	Skipped  []string
	Conflict *Conflict
}

// envMetas devolve a metainformação dos commits do ambiente desde o merge-base
// com a produção (ou seja, os commits aplicados sobre produção).
func (a *App) envMetas(envBranch string) ([]CommitMeta, error) {
	prod := "origin/" + a.Repo.Production
	mb, err := a.Git.MergeBase(prod, "origin/"+envBranch)
	if err != nil {
		return nil, err
	}
	commits, err := a.Git.Log(mb+"..origin/"+envBranch, false)
	if err != nil {
		return nil, err
	}
	metas := make([]CommitMeta, 0, len(commits))
	for _, c := range commits {
		metas = append(metas, parseMeta(c))
	}
	return metas, nil
}

// featureCommits devolve (merge-base, SHAs em ordem) do intervalo produção..feature,
// recusando se houver merge commits.
func (a *App) featureCommits(featureRef string) (string, []string, error) {
	prod := "origin/" + a.Repo.Production
	mb, err := a.Git.MergeBase(prod, featureRef)
	if err != nil {
		return "", nil, fmt.Errorf("não foi possível achar o merge-base com %s: %w", a.Repo.Production, err)
	}
	rng := mb + ".." + featureRef
	hasMerges, err := a.Git.HasMerges(rng)
	if err != nil {
		return "", nil, err
	}
	if hasMerges {
		return "", nil, fmt.Errorf("a branch tem merge commits no intervalo %s..feature; faça `git rebase %s` para linearizar", a.Repo.Production, a.Repo.Production)
	}
	shas, err := a.Git.RevListRange(rng, true, true)
	if err != nil {
		return "", nil, err
	}
	return mb, shas, nil
}

// missing calcula quais commits da feature ainda não estão no ambiente.
func (a *App) missing(featureRef, envBranch string) (pending []string, skipped []string, err error) {
	mb, shas, err := a.featureCommits(featureRef)
	if err != nil {
		return nil, nil, err
	}
	// Equivalência por patch-id (git cherry).
	patchApplied, err := a.Git.CherryEquivalent("origin/"+envBranch, featureRef, mb)
	if err != nil {
		return nil, nil, err
	}
	// SHAs de origem registrados por "(cherry picked from commit X)" no ambiente.
	metas, err := a.envMetas(envBranch)
	if err != nil {
		return nil, nil, err
	}
	for _, sha := range shas {
		applied := patchApplied[sha]
		if !applied {
			for _, m := range metas {
				if m.CherryFrom != "" && matchesSHA(m.CherryFrom, sha) {
					applied = true
					break
				}
			}
		}
		if applied {
			skipped = append(skipped, sha)
		} else {
			pending = append(pending, sha)
		}
	}
	return pending, skipped, nil
}

// worktreePath devolve o caminho do worktree efêmero para aplicar em um ambiente.
func (a *App) worktreePath(prefix, logical string, pr int) (string, error) {
	common, err := a.Git.CommonDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(common, "flow", fmt.Sprintf("%s-%s-%d", prefix, logical, pr)), nil
}

// applyToEnv aplica, em um worktree efêmero, os commits pendentes da feature no
// ambiente e faz push com retry. Devolve ApplyOutcome; Conflict != nil quando
// um cherry-pick (ou o rebase de push) conflita.
func (a *App) applyToEnv(env Env, pr int, featureRef string) (*ApplyOutcome, error) {
	if err := a.Git.Fetch(); err != nil {
		return nil, fmt.Errorf("falha no fetch: %w", err)
	}

	pending, skipped, err := a.missing(featureRef, env.Branch)
	if err != nil {
		return nil, err
	}
	out := &ApplyOutcome{Skipped: skipped}
	if len(pending) == 0 {
		return out, nil
	}

	wtPath, err := a.worktreePath("apply", env.Logical, pr)
	if err != nil {
		return nil, err
	}
	workBranch := fmt.Sprintf("flow-apply-%s-%d", env.Logical, pr)
	a.Git.WorktreeRemove(wtPath)
	if err := a.Git.WorktreeAdd(wtPath, workBranch, "origin/"+env.Branch); err != nil {
		return nil, fmt.Errorf("não foi possível criar worktree: %w", err)
	}
	defer a.Git.WorktreeRemove(wtPath)
	wt := a.Git.At(wtPath)

	for i, sha := range pending {
		if err := wt.CherryPick(sha); err != nil {
			files, _ := wt.UnmergedFiles()
			wt.CherryPickAbort()
			out.Conflict = &Conflict{
				Env: env, PR: pr, Phase: "cherry-pick",
				FailedSHA: sha, Files: files, Pending: pending[i:],
			}
			return out, nil
		}
		if err := wt.AmendTrailers(trailerFlowPR(pr)); err != nil {
			return nil, fmt.Errorf("não foi possível gravar trailer Flow-PR: %w", err)
		}
		out.Applied = append(out.Applied, sha)
	}

	if conf, err := pushWithRetry(wt, workBranch, env.Branch, env, pr); err != nil {
		return nil, err
	} else if conf != nil {
		out.Conflict = conf
	}
	return out, nil
}

// pushWithRetry faz push de localBranch -> envBranch com até 3 tentativas,
// rebaseando sobre origin/<envBranch> em caso de rejeição por concorrência.
// Conflito no rebase vira um Conflict (fase "rebase") para diagnóstico.
func pushWithRetry(wt *git.Git, localBranch, envBranch string, env Env, pr int) (*Conflict, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if err := wt.Push(localBranch+":"+envBranch, false); err == nil {
			return nil, nil
		} else {
			lastErr = err
		}
		if err := wt.Fetch(envBranch); err != nil {
			return nil, fmt.Errorf("falha no fetch durante retry de push: %w", err)
		}
		if err := wt.Rebase("origin/" + envBranch); err != nil {
			files, _ := wt.UnmergedFiles()
			wt.RebaseAbort()
			return &Conflict{Env: env, PR: pr, Phase: "rebase", Files: files}, nil
		}
	}
	return nil, fmt.Errorf("push para %s rejeitado após 3 tentativas: %w", envBranch, lastErr)
}

func trailerFlowPR(pr int) string { return "Flow-PR: #" + strconv.Itoa(pr) }

func trailerFlowDrop(pr int) string { return "Flow-Drop: #" + strconv.Itoa(pr) }

func trailerFlowResolves(prs []int) string {
	parts := make([]string, len(prs))
	for i, p := range prs {
		parts[i] = "#" + strconv.Itoa(p)
	}
	return "Flow-Resolves: " + strings.Join(parts, ", ")
}
