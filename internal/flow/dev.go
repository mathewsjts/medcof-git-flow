package flow

import (
	"fmt"

	"github.com/mathewsjts/medcof-git-flow/internal/github"
)

// Dev aplica os commits faltantes do PR da branch atual no ambiente dev.
func (a *App) Dev() error {
	pr, err := a.GH.CurrentPR()
	if err != nil {
		return fmt.Errorf("não foi possível descobrir o PR da branch atual: %w", err)
	}
	if err := a.requireBaseProduction(pr); err != nil {
		return err
	}
	env, err := a.Env("dev")
	if err != nil {
		return err
	}
	return a.applyAndReport(env, pr)
}

// requireBaseProduction garante que o PR tem base na branch de produção e está aberto.
func (a *App) requireBaseProduction(pr github.PR) error {
	if pr.State == "MERGED" || pr.Merged {
		return fmt.Errorf("PR #%d já foi mergeado", pr.Number)
	}
	if pr.State == "CLOSED" {
		return fmt.Errorf("PR #%d está fechado", pr.Number)
	}
	if pr.BaseRefName != a.Repo.Production {
		return fmt.Errorf("PR #%d tem base %q; o flow exige base %q", pr.Number, pr.BaseRefName, a.Repo.Production)
	}
	return nil
}

// applyAndReport executa applyToEnv e imprime o resultado, delegando conflito ao diagnóstico.
func (a *App) applyAndReport(env Env, pr github.PR) error {
	a.UI.Step("Aplicando PR #%d em %s...", pr.Number, env)
	featureRef := "origin/" + pr.HeadRefName
	out, err := a.applyToEnv(env, pr.Number, featureRef)
	if err != nil {
		return err
	}
	if out.Conflict != nil {
		return a.reportConflict(out.Conflict)
	}
	if len(out.Applied) == 0 {
		a.UI.OK("%s já está atualizado com o PR #%d (nada a aplicar)", env, pr.Number)
		return nil
	}
	a.UI.OK("%d commit(s) aplicado(s) em %s (PR #%d)", len(out.Applied), env, pr.Number)
	if len(out.Skipped) > 0 {
		a.UI.Info("  %d commit(s) já estavam aplicados", len(out.Skipped))
	}
	return nil
}
