package flow

import "fmt"

// Status mostra, para cada ambiente, quantos commits do alvo já estão lá e
// quais faltam.
func (a *App) Status(arg string) error {
	if err := a.Git.Fetch(); err != nil {
		return fmt.Errorf("falha no fetch: %w", err)
	}
	t, err := a.resolveTarget(arg)
	if err != nil {
		return err
	}
	if t.PR != nil {
		a.UI.Info("PR #%d (%s): %s, %s", t.PR.Number, t.Branch, t.PR.Title, reviewLabel(*t.PR))
	} else {
		a.UI.Info("%s (sem PR aberto para %s)", t.Branch, a.Repo.Production)
	}

	commits, err := a.featureCommits(t.Ref())
	if err != nil {
		return err
	}
	if len(commits) == 0 {
		a.UI.Info("Nenhum commit em relação a %s.", a.Repo.Production)
		return nil
	}
	for _, logical := range a.Repo.Envs() {
		env, err := a.Env(logical)
		if err != nil {
			return err
		}
		pending, _, err := a.missing(commits, t.Ref(), env.Branch)
		if err != nil {
			return err
		}
		done := len(commits) - len(pending)
		switch {
		case len(pending) == 0:
			a.UI.OK("%s: aplicado (%d/%d)", env, done, len(commits))
		case done == 0:
			a.UI.Fail("%s: ausente (0/%d)", env, len(commits))
		default:
			a.UI.Warn("%s: parcial (%d/%d); faltam:", env, done, len(commits))
			for _, c := range pending {
				a.UI.Info("    %s %s", c.Short(), c.Subject())
			}
		}
	}
	return nil
}
