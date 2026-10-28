package flow

import (
	"fmt"

	"github.com/mathewsjts/medcof-git-flow/internal/github"
)

// Hom aplica em hom os commits do alvo que ainda não estão lá. Exige PR aberto
// para produção com ao menos um approve e sem mudanças pedidas; não exige as
// regras de merge do GitHub (mínimo de aprovações, code owner).
func (a *App) Hom(arg string) error {
	env, t, err := a.prepare("hom", arg)
	if err != nil {
		return err
	}
	if t.PR == nil {
		return fmt.Errorf("%s não tem PR aberto para %s; flow hom exige PR aprovado", t.Branch, a.Repo.Production)
	}
	if !approved(*t.PR) {
		return fmt.Errorf("PR #%d ainda não está aprovado (%s); flow hom exige PR aprovado", t.PR.Number, reviewLabel(*t.PR))
	}
	return a.propagate(env, t)
}

// approved diz se o PR tem ao menos um approve e nenhum pedido de mudança.
func approved(pr github.PR) bool {
	if pr.ReviewDecision == "CHANGES_REQUESTED" {
		return false
	}
	return pr.ReviewDecision == "APPROVED" || pr.Approvals > 0
}

// reviewLabel traduz o estado de review do PR.
func reviewLabel(pr github.PR) string {
	switch {
	case pr.ReviewDecision == "CHANGES_REQUESTED":
		return "mudanças pedidas"
	case pr.ReviewDecision == "APPROVED":
		return "aprovado"
	case pr.ReviewDecision == "REVIEW_REQUIRED" && pr.Approvals > 0:
		return fmt.Sprintf("%d aprovação(ões); o merge ainda exige mais reviews", pr.Approvals)
	case pr.Approvals > 0:
		return "aprovado"
	default:
		return "aguardando review"
	}
}
