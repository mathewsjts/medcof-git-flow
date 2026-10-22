package flow

import (
	"fmt"

	"github.com/mathewsjts/medcof-git-flow/internal/git"
)

// openConflict reaplica os commits pendentes no working tree do usuário, na
// branch do ambiente alinhada a origin, e para no cherry-pick que conflita.
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
		ahead, err := a.Git.RevListRange("origin/"+env.Branch+".."+env.Branch, false, false)
		if err != nil {
			return err
		}
		if len(ahead) > 0 {
			return fmt.Errorf("sua %s local tem %d commit(s) que não estão em origin/%s; envie ou descarte antes, porque o conflito é aberto com %s alinhada a origin", env.Branch, len(ahead), env.Branch, env.Branch)
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
