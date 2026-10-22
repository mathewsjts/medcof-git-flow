package flow

import (
	"fmt"
	"time"

	"github.com/mathewsjts/medcof-git-flow/internal/git"
)

// openConflict reaplica os commits pendentes no working tree do usuário, na
// branch do ambiente alinhada a origin, e para no cherry-pick que conflita. A
// branch local do ambiente é descartada; commits dela que não estão em nenhuma
// branch do origin ficam guardados numa branch flow-backup/.
// Daí em diante é git puro: resolver, `git cherry-pick --continue` e rodar o
// comando de novo, que envia a branch do ambiente (veja pushResolution).
func (a *App) openConflict(env Env, t Target, pending []git.LogCommit, failed git.LogCommit) error {
	rerun := fmt.Sprintf("flow %s %s", env.Logical, t.Arg())
	a.UI.Fail("Conflito ao aplicar %s em %s, no commit %s %s", t, env, failed.Short(), failed.Subject())

	dirty, err := a.Git.TrackedChanges()
	if err != nil {
		return err
	}
	if dirty {
		return fmt.Errorf("para abrir o conflito o working tree não pode ter mudanças em arquivos rastreados: faça commit ou `git stash` e rode `%s` de novo", rerun)
	}
	if a.Git.BranchExists(env.Branch) {
		local, err := a.Git.LocalOnly(env.Branch)
		if err != nil {
			return err
		}
		if len(local) > 0 {
			backup := fmt.Sprintf("flow-backup/%s-%s", env.Branch, time.Now().Format("20060102-150405"))
			if err := a.Git.CreateBranch(backup, env.Branch); err != nil {
				return err
			}
			a.UI.Warn("sua %s local tinha %d commit(s) que não estão no origin; guardei em %s antes de alinhar a origin/%s", env.Branch, len(local), backup, env.Branch)
		}
	}

	if err := a.Git.SwitchReset(env.Branch, "origin/"+env.Branch); err != nil {
		return err
	}
	shas := make([]string, len(pending))
	for i, c := range pending {
		shas[i] = c.SHA
	}
	if err := a.Git.CherryPick(shas...); err == nil {
		// origin mudou entre a tentativa no worktree e aqui; nada a resolver.
		return fmt.Errorf("os commits entraram sem conflito na sua %s local; rode `%s` de novo para enviar", env.Branch, rerun)
	}

	files, _ := a.Git.UnmergedFiles()
	a.UI.Info("  Abri o conflito no seu working tree: você está em %s, alinhada a origin/%s.", env.Branch, env.Branch)
	if len(files) > 0 {
		a.UI.Info("  Arquivos em conflito:")
		for _, f := range files {
			a.UI.Info("    %s", f)
		}
	}
	a.UI.Info("")
	a.UI.Info("  Para resolver:")
	a.UI.Info("    # edite os arquivos e remova os marcadores de conflito")
	a.UI.Info("    git add <arquivos>")
	a.UI.Info("    git cherry-pick --continue   # segue com os commits restantes (pode parar de novo)")
	a.UI.Info("    %s   # envia %s e aplica o que faltar", rerun, env.Branch)
	a.UI.Info("")
	a.UI.Info("  Para desistir: git cherry-pick --abort (nada foi enviado ao remoto)")
	return fmt.Errorf("conflito aberto em %s: resolva e rode `%s`", env.Branch, rerun)
}
