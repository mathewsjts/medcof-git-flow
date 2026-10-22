package flow

import "fmt"

// Hom aplica em hom os commits do alvo que ainda não estão lá. Exige PR aberto
// para produção e aprovado.
func (a *App) Hom(arg string) error {
	env, t, err := a.prepare("hom", arg)
	if err != nil {
		return err
	}
	if t.PR == nil {
		return fmt.Errorf("%s não tem PR aberto para %s; flow hom exige PR aprovado", t.Branch, a.Repo.Production)
	}
	if t.PR.ReviewDecision != "APPROVED" {
		return fmt.Errorf("PR #%d ainda não está aprovado (%s); flow hom exige PR aprovado", t.PR.Number, reviewLabel(t.PR.ReviewDecision))
	}
	return a.propagate(env, t)
}

// reviewLabel traduz o reviewDecision do GitHub.
func reviewLabel(decision string) string {
	switch decision {
	case "APPROVED":
		return "aprovado"
	case "CHANGES_REQUESTED":
		return "mudanças pedidas"
	default:
		return "aguardando review"
	}
}
