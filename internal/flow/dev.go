package flow

import "fmt"

// Dev aplica em dev os commits do alvo (PR, branch ou branch atual) que ainda
// não estão lá.
func (a *App) Dev(arg string) error {
	env, t, err := a.prepare("dev", arg)
	if err != nil {
		return err
	}
	return a.propagate(env, t)
}

// prepare resolve o ambiente, atualiza o origin e resolve o alvo.
func (a *App) prepare(logical, arg string) (Env, Target, error) {
	env, err := a.Env(logical)
	if err != nil {
		return Env{}, Target{}, err
	}
	if err := a.Git.Fetch(); err != nil {
		return Env{}, Target{}, fmt.Errorf("falha no fetch: %w", err)
	}
	t, err := a.resolveTarget(arg)
	if err != nil {
		return Env{}, Target{}, err
	}
	return env, t, nil
}

// propagate envia uma resolução de conflito pendente, se houver, e aplica no
// ambiente os commits do alvo que faltam. Num conflito, abre-o no working tree.
func (a *App) propagate(env Env, t Target) error {
	if a.Git.CherryPickInProgress() {
		return fmt.Errorf("há um cherry-pick em andamento neste repositório: termine com `git cherry-pick --continue` (ou desista com `--abort`) e rode de novo")
	}
	a.UI.Step("Aplicando %s em %s...", t, env)
	commits, err := a.featureCommits(t.Ref())
	if err != nil {
		return err
	}

	sent, err := a.pushResolution(env, commits)
	if err != nil {
		return err
	}
	if sent > 0 {
		a.UI.OK("%d commit(s) da sua resolução enviados para %s", sent, env)
		if cur, _ := a.Git.CurrentBranch(); cur == env.Branch {
			a.UI.Info("  Você continua em %s; `git switch %s` para voltar.", env.Branch, t.Branch)
		}
	}

	pending, applied, err := a.missing(commits, t.Ref(), env.Branch)
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		if sent == 0 {
			a.UI.OK("%s já está atualizado com %s (nada a aplicar)", env, t)
		}
		return nil
	}

	failed, err := a.applyInWorktree(env, pending)
	if err != nil {
		return err
	}
	if failed != nil {
		return a.openConflict(env, t, pending, *failed)
	}
	a.UI.OK("%d commit(s) aplicado(s) em %s (%s)", len(pending), env, t)
	if len(applied) > 0 {
		a.UI.Info("  %d commit(s) já estavam aplicados", len(applied))
	}
	return nil
}
