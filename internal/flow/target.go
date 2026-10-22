package flow

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/mathewsjts/medcof-git-flow/internal/github"
)

// Target é a branch de feature que será propagada, com o PR dela para a
// produção quando existir.
type Target struct {
	Branch string
	PR     *github.PR // nil quando a branch não tem PR aberto para produção
}

// Ref é a referência remota de onde os commits são lidos.
func (t Target) Ref() string { return "origin/" + t.Branch }

// String descreve o alvo como "PR #123 (feature/x)" ou "feature/x".
func (t Target) String() string {
	if t.PR != nil {
		return fmt.Sprintf("PR #%d (%s)", t.PR.Number, t.Branch)
	}
	return t.Branch
}

// Arg é o argumento que identifica o alvo nos comandos sugeridos ao usuário.
func (t Target) Arg() string {
	if t.PR != nil {
		return strconv.Itoa(t.PR.Number)
	}
	return t.Branch
}

var rePRURL = regexp.MustCompile(`^https?://github\.com/([^/]+/[^/]+)/pull/(\d+)`)

// prNumber reconhece um PR no argumento: "123", "#123" ou a URL do PR (que
// precisa ser deste repositório).
func (a *App) prNumber(arg string) (int, bool, error) {
	if m := rePRURL.FindStringSubmatch(arg); m != nil {
		if !strings.EqualFold(m[1], a.Repo.Name) {
			return 0, false, fmt.Errorf("o PR %s é de %s, mas este repositório é %s", arg, m[1], a.Repo.Name)
		}
		n, _ := strconv.Atoi(m[2])
		return n, true, nil
	}
	n, err := strconv.Atoi(strings.TrimPrefix(arg, "#"))
	return n, err == nil, nil
}

// resolveTarget interpreta o argumento de dev/hom/status: PR ("123", "#123" ou
// a URL), nome de branch ou, vazio, a branch atual. Espera o fetch já feito.
func (a *App) resolveTarget(arg string) (Target, error) {
	n, isPR, err := a.prNumber(arg)
	if err != nil {
		return Target{}, err
	}
	if isPR {
		pr, err := a.GH.PR(n)
		if err != nil {
			return Target{}, err
		}
		if pr.State != "OPEN" {
			return Target{}, fmt.Errorf("PR #%d não está aberto (%s)", pr.Number, strings.ToLower(pr.State))
		}
		if pr.BaseRefName != a.Repo.Production {
			return Target{}, fmt.Errorf("PR #%d tem base %q; o flow exige base %q", pr.Number, pr.BaseRefName, a.Repo.Production)
		}
		return a.checkTarget(Target{Branch: pr.HeadRefName, PR: &pr})
	}

	branch := arg
	if branch == "" {
		b, err := a.Git.CurrentBranch()
		if err != nil {
			return Target{}, err
		}
		if b == "HEAD" {
			return Target{}, fmt.Errorf("HEAD destacado: informe o PR ou a branch")
		}
		branch = b
	}
	if branch == a.Repo.Production || a.isEnvBranch(branch) {
		return Target{}, fmt.Errorf("%q não é uma branch de feature: informe o PR ou a branch (ex.: `flow dev 123`)", branch)
	}
	pr, found, err := a.GH.PRForBranch(branch, a.Repo.Production)
	if err != nil {
		return Target{}, err
	}
	t := Target{Branch: branch}
	if found {
		t.PR = &pr
	}
	return a.checkTarget(t)
}

// checkTarget confirma que a branch está no origin e avisa sobre commits locais
// não enviados, que não entram na propagação.
func (a *App) checkTarget(t Target) (Target, error) {
	if _, err := a.Git.RevParse(t.Ref()); err != nil {
		return Target{}, fmt.Errorf("branch %s não existe em origin: faça `git push -u origin %s`", t.Branch, t.Branch)
	}
	if a.Git.BranchExists(t.Branch) {
		ahead, err := a.Git.RevListRange(t.Ref()+".."+t.Branch, false, false)
		if err != nil {
			return Target{}, err
		}
		if len(ahead) > 0 {
			a.UI.Warn("sua %s local tem %d commit(s) que não estão em origin e não serão aplicados (faça push)", t.Branch, len(ahead))
		}
	}
	return t, nil
}

func (a *App) isEnvBranch(branch string) bool {
	for _, b := range a.Repo.Environments {
		if b == branch {
			return true
		}
	}
	return false
}
